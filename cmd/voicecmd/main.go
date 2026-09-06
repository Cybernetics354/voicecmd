package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "net/http/pprof"
	"voicecmd/internal/audio"
	"voicecmd/internal/config"
	"voicecmd/internal/executor"
	"voicecmd/internal/hotkey"
	"voicecmd/internal/matcher"
	"voicecmd/internal/stt"
)

var Version = "1.0.0"

type App struct {
	cfg       *config.Config
	recorder  *audio.Recorder
	sttEngine *stt.Engine
	listener  *hotkey.Listener
	ipcServer *hotkey.IPCServer

	mu         sync.Mutex
	isHandling bool
}

func main() {
	configPath := flag.String(
		"config",
		"",
		"Path to YAML configuration file (default: config/config.yaml)",
	)
	listDevicesFlag := flag.Bool(
		"list-devices",
		false,
		"List available audio and keyboard input devices",
	)
	transcribeFlag := flag.String(
		"transcribe",
		"",
		"Transcribe an existing WAV file and test command matching",
	)
	triggerFlag := flag.String(
		"trigger",
		"",
		"Send command to running voicecmd daemon via IPC: start, stop, or status",
	)
	pttFlag := flag.Bool(
		"ptt",
		false,
		"Interactive push-to-talk in terminal (Press Enter to start/stop)",
	)
	testMicFlag := flag.Int(
		"test-mic",
		0,
		"Record audio for N seconds from microphone and save to test.wav",
	)
	versionFlag := flag.Bool("version", false, "Print version and exit")

	flag.Parse()

	if *versionFlag {
		fmt.Printf("VoiceCmd v%s (Whisper.cpp Voice Controller for Linux)\n", Version)
		return
	}

	if *triggerFlag != "" {
		handleTriggerCommand(*triggerFlag)
		return
	}

	if *listDevicesFlag {
		handleListDevices()
		return
	}

	if *testMicFlag > 0 {
		handleTestMic(*testMicFlag, *configPath)
		return
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Direct transcription test mode
	if *transcribeFlag != "" {
		handleTranscribeFile(cfg, *transcribeFlag)
		return
	}

	// Terminal Push-To-Talk interactive mode
	if *pttFlag {
		runInteractivePTT(cfg)
		return
	}

	go http.ListenAndServe("localhost:6060", nil)

	// Run full daemon mode
	runDaemon(cfg)
}

func handleTriggerCommand(cmd string) {
	resp, err := hotkey.SendIPCCommand(hotkey.DefaultSocketPath, cmd)
	if err != nil {
		log.Fatalf("IPC error: %v", err)
	}
	fmt.Println(resp)
}

func handleListDevices() {
	fmt.Println("=== Audio Input Devices ===")
	audioDevs, err := audio.ListDevices()
	if err != nil {
		fmt.Printf("Error querying audio devices: %v\n", err)
	} else {
		for i, d := range audioDevs {
			fmt.Printf("  [%d] %s\n", i, d)
		}
	}
	_ = audio.Terminate()

	fmt.Println("\n=== Keyboard Input Devices ===")
	kbs, err := hotkey.FindKeyboards()
	if err != nil {
		fmt.Printf("Error querying keyboards: %v\n", err)
	} else if len(kbs) == 0 {
		fmt.Println("  No keyboards detected (or permission denied)")
	} else {
		for _, kb := range kbs {
			fmt.Printf("  - %s (%s)\n", kb.Name, kb.Path)
		}
	}
}

func handleTestMic(durationSeconds int, configPath string) {
	cfg, _ := config.Load(configPath)
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	fmt.Printf("Recording %d seconds of audio to test.wav...\n", durationSeconds)
	rec := audio.NewRecorder(cfg.Audio.Device, cfg.Audio.SampleRate, cfg.Audio.Channels)
	if err := rec.Start(); err != nil {
		log.Fatalf("Failed to start recording: %v", err)
	}

	time.Sleep(time.Duration(durationSeconds) * time.Second)

	samples, dur, err := rec.Stop()
	if err != nil {
		log.Fatalf("Recording error: %v", err)
	}
	_ = audio.Terminate()

	if err := audio.WriteWAV(
		"test.wav",
		samples,
		cfg.Audio.SampleRate,
		cfg.Audio.Channels,
	); err != nil {
		log.Fatalf("Failed to save test.wav: %v", err)
	}

	fmt.Printf("Saved test.wav (recorded %.2f seconds, %d samples)\n", dur.Seconds(), len(samples))
}

func handleTranscribeFile(cfg *config.Config, wavPath string) {
	fmt.Printf("Initializing Whisper engine (model: %s)...\n", cfg.STT.ModelPath)
	engine, err := stt.NewEngine(
		cfg.STT.BinaryPath,
		cfg.STT.ModelPath,
		cfg.STT.Language,
		cfg.STT.Threads,
	)
	if err != nil {
		log.Fatalf("Failed to init STT: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Printf("Transcribing %s...\n", wavPath)
	start := time.Now()
	res, err := engine.Transcribe(ctx, wavPath)
	if err != nil {
		log.Fatalf("Transcription failed: %v", err)
	}

	fmt.Printf("Raw:   %q\n", res.RawText)
	fmt.Printf("Clean: %q\n", res.CleanText)
	fmt.Printf("Time:  %v\n\n", time.Since(start))

	match := matcher.Match(res.CleanText, cfg.Commands, cfg.STT.Threshold)
	if match != nil {
		fmt.Printf("Matched Command: %q\n", match.CommandName)
		fmt.Printf("Matched Phrase:  %q (Score: %.2f)\n", match.MatchedPhrase, match.Score)
		fmt.Printf(
			"Program:         %s %v\n",
			match.CommandConfig.Command.Program,
			match.CommandConfig.Command.Args,
		)
	} else {
		fmt.Println("No matching command found above threshold.")
	}
}

func runInteractivePTT(cfg *config.Config) {
	fmt.Println("=== VoiceCmd Interactive Push-To-Talk Mode ===")
	fmt.Println(
		"Press [ENTER] to start recording, speak your command, then press [ENTER] again to stop.",
	)
	fmt.Println("Type 'quit' or press Ctrl+C to exit.")
	fmt.Println()

	engine, err := stt.NewEngine(
		cfg.STT.BinaryPath,
		cfg.STT.ModelPath,
		cfg.STT.Language,
		cfg.STT.Threads,
	)
	if err != nil {
		log.Fatalf("Failed to init STT: %v", err)
	}

	rec := audio.NewRecorder(cfg.Audio.Device, cfg.Audio.SampleRate, cfg.Audio.Channels)
	scanner := bufio.NewScanner(os.Stdin)
	if err := scanner.Err(); err != nil {
		fmt.Print(err)
		return
	}

	for {
		fmt.Print("Press [ENTER] to record: ")
		if !scanner.Scan() {
			break
		}
		if scanner.Text() == "quit" {
			break
		}

		if err := rec.Start(); err != nil {
			fmt.Printf("Error starting recording: %v\n", err)
			continue
		}

		fmt.Print("🔴 Recording... Press [ENTER] to stop: ")
		if !scanner.Scan() {
			_, _, _ = rec.Stop()
			break
		}

		samples, dur, err := rec.Stop()
		if err != nil {
			fmt.Printf("Error stopping recording: %v\n", err)
			continue
		}

		if dur < 300*time.Millisecond {
			fmt.Printf("⚠️  Recording too short (%.2fs), ignored.\n\n", dur.Seconds())
			continue
		}

		processRecordedAudio(cfg, engine, samples)
	}

	_ = audio.Terminate()
}

func runDaemon(cfg *config.Config) {
	fmt.Println("==================================================")
	fmt.Println("  VoiceCmd - Voice Activated Command Controller   ")
	fmt.Printf("  Version: %s\n", Version)
	fmt.Printf("  Hotkey:  %s\n", cfg.Hotkey.Key)
	fmt.Printf("  STT:     %s (lang: %s)\n", cfg.STT.ModelPath, cfg.STT.Language)
	fmt.Printf("  Loaded:  %d voice commands\n", len(cfg.Commands))
	fmt.Println("==================================================")

	engine, err := stt.NewEngine(
		cfg.STT.BinaryPath,
		cfg.STT.ModelPath,
		cfg.STT.Language,
		cfg.STT.Threads,
	)
	if err != nil {
		log.Fatalf("Failed to initialize STT engine: %v", err)
	}

	rec := audio.NewRecorder(cfg.Audio.Device, cfg.Audio.SampleRate, cfg.Audio.Channels)

	app := &App{
		cfg:       cfg,
		recorder:  rec,
		sttEngine: engine,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutdown signal received. Exiting...")
		cancel()
		_ = audio.Terminate()
		os.Exit(0)
	}()

	// Start IPC Unix Socket Server for external triggers
	app.ipcServer = hotkey.NewIPCServer(
		hotkey.DefaultSocketPath,
		app.OnPress,
		app.OnRelease,
		func() string {
			if app.recorder.IsRecording() {
				return "recording"
			}
			return "idle"
		},
	)
	go func() {
		if err := app.ipcServer.Start(ctx); err != nil {
			log.Printf("[IPC] Socket server stopped: %v", err)
		}
	}()
	fmt.Printf("  IPC Socket active at %s\n", hotkey.DefaultSocketPath)

	// Start Evdev keyboard hotkey listener
	listener, err := hotkey.NewListener(
		cfg.Hotkey.Key,
		cfg.Hotkey.Device,
		app.OnPress,
		app.OnRelease,
	)
	if err != nil {
		log.Fatalf("Failed to create hotkey listener: %v", err)
	}
	app.listener = listener

	fmt.Println("\nListening for hotkey... Hold key combo to record, release to transcribe.")
	if err := listener.Start(ctx); err != nil {
		fmt.Printf("\n⚠️  Evdev hotkey listener error: %v\n", err)
		fmt.Println(
			"\nNote: You can also use 'voicecmd -ptt' for interactive push-to-talk in terminal,",
		)
		fmt.Printf(
			"or trigger via IPC socket using 'voicecmd -trigger start' / 'voicecmd -trigger stop'.\n",
		)
		// Keep daemon running for IPC socket even if evdev is blocked
		<-ctx.Done()
	}
}

func (a *App) OnPress() {
	a.mu.Lock()
	if a.isHandling || a.recorder.IsRecording() {
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()

	fmt.Println("\n🔴 [RECORDING] Combination pressed. Listening...")
	if err := a.recorder.Start(); err != nil {
		log.Printf("Failed to start recording: %v", err)
	}
}

func (a *App) OnRelease() {
	if !a.recorder.IsRecording() {
		return
	}

	samples, dur, err := a.recorder.Stop()
	if err != nil {
		log.Printf("Error stopping recording: %v", err)
		return
	}

	if dur < 300*time.Millisecond {
		fmt.Printf(
			"⚠️  [CANCELLED] Key held for only %.2fs (<0.3s), ignoring tap.\n",
			dur.Seconds(),
		)
		return
	}

	fmt.Printf(
		"⏹️  [RECORDED] Recorded %.2f seconds (%d samples). Transcribing...\n",
		dur.Seconds(),
		len(samples),
	)

	a.mu.Lock()
	a.isHandling = true
	a.mu.Unlock()

	go func() {
		defer func() {
			a.mu.Lock()
			a.isHandling = false
			a.mu.Unlock()
		}()

		processRecordedAudio(a.cfg, a.sttEngine, samples)
	}()
}

func processRecordedAudio(cfg *config.Config, engine *stt.Engine, samples []float32) {
	wavPath := cfg.STT.WavPath
	if wavPath == "" {
		wavPath = "/tmp/voicecmd_recording.wav"
	}

	if err := audio.WriteWAV(
		wavPath,
		samples,
		cfg.Audio.SampleRate,
		cfg.Audio.Channels,
	); err != nil {
		log.Printf("Failed to write WAV file: %v", err)
		return
	}
	if !cfg.STT.KeepWav {
		defer os.Remove(wavPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	start := time.Now()
	res, err := engine.Transcribe(ctx, wavPath)
	if err != nil {
		log.Printf("Transcription error: %v", err)
		return
	}

	elapsed := time.Since(start)

	if res.CleanText == "" {
		fmt.Printf(
			"⚠️  [NO SPEECH] No intelligible speech detected (raw: %q, took %v)\n",
			res.RawText,
			elapsed,
		)
		return
	}

	fmt.Printf("📝 [TRANSCRIPTION] %q (took %v)\n", res.CleanText, elapsed)

	match := matcher.Match(res.CleanText, cfg.Commands, cfg.STT.Threshold)
	if match == nil {
		fmt.Printf(
			"❓ [NO MATCH] No command matched %q (threshold: %.2f)\n",
			res.CleanText,
			cfg.STT.Threshold,
		)
		return
	}

	fmt.Printf(
		"🎯 [MATCHED] Action: %q | Phrase: %q | Score: %.2f\n",
		match.CommandName,
		match.MatchedPhrase,
		match.Score,
	)
	fmt.Printf(
		"🚀 [EXECUTING] %s %v\n",
		match.CommandConfig.Command.Program,
		match.CommandConfig.Command.Args,
	)

	execRes := executor.Execute(ctx, match.CommandConfig, res.CleanText)
	if execRes.Err != nil {
		fmt.Printf("❌ [EXEC ERROR] %v\n", execRes.Err)
		if execRes.Stderr != "" {
			fmt.Printf("   Stderr: %s\n", execRes.Stderr)
		}
	} else {
		fmt.Println("✅ [SUCCESS] Command executed successfully")
		if execRes.Stdout != "" {
			fmt.Printf("   Stdout: %s\n", execRes.Stdout)
		}
	}
}
