package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"voicecmd/internal/audio"
	"voicecmd/internal/config"
	"voicecmd/internal/executor"
	"voicecmd/internal/hotkey"
	"voicecmd/internal/matcher"
	"voicecmd/internal/stt"
	"voicecmd/internal/tray"
)

var Version = "1.0.0"

type App struct {
	cfg                *config.Config
	recorder           *audio.Recorder
	sttEngine          *stt.Engine
	listener           *hotkey.Listener
	ipcServer          *hotkey.IPCServer
	trayMgr            *tray.Manager
	fallbackConfigPath string
	currentConfigPath  string

	mu         sync.Mutex
	isHandling bool
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "trigger" {
		triggerArgs := os.Args[2:]
		if len(triggerArgs) == 0 {
			log.Fatal("Error: trigger command required: start, stop, status, or load [path]")
		}
		handleTriggerCommand(strings.Join(triggerArgs, " "))
		return
	}

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
		"Send command to running voicecmd daemon via IPC: start, stop, status, or load [path]",
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
	noTrayFlag := flag.Bool(
		"no-tray",
		false,
		"Disable system tray status icon and context menu",
	)
	versionFlag := flag.Bool("version", false, "Print version and exit")

	flag.Parse()

	if *versionFlag {
		fmt.Printf("VoiceCmd v%s (Whisper.cpp Voice Controller for Linux)\n", Version)
		return
	}

	if *triggerFlag != "" {
		cmd := *triggerFlag
		if len(flag.Args()) > 0 {
			cmd = cmd + " " + strings.Join(flag.Args(), " ")
		}
		handleTriggerCommand(cmd)
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

	fallbackPath := *configPath
	if fallbackPath == "" {
		fallbackPath = cfg.ConfigPath
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

	// Run full daemon mode
	runDaemon(cfg, fallbackPath, !*noTrayFlag)
}

func handleTriggerCommand(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) > 0 {
		sub := strings.ToLower(parts[0])
		if (sub == "load" || sub == "reload") && len(parts) > 1 {
			target := strings.TrimSpace(cmd[len(parts[0]):])
			target = strings.Trim(target, `"'`)
			if target != "" {
				target = config.ExpandPath(target)
				if abs, err := filepath.Abs(target); err == nil {
					cmd = fmt.Sprintf("%s %s", sub, abs)
				}
			}
		}
	}

	resp, err := hotkey.SendIPCCommand(hotkey.DefaultSocketPath, cmd)
	if err != nil {
		log.Fatalf("IPC error: %v", err)
	}
	fmt.Println(resp)
	if strings.HasPrefix(resp, "ERR:") {
		os.Exit(1)
	}
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

func runDaemon(cfg *config.Config, fallbackPath string, enableTray bool) {
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
		cfg:                cfg,
		recorder:           rec,
		sttEngine:          engine,
		fallbackConfigPath: fallbackPath,
		currentConfigPath:  cfg.ConfigPath,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize System Tray if enabled
	trayEnabled := enableTray && cfg.Tray.IsEnabled()
	if trayEnabled {
		if !tray.IsDBusAvailable() {
			fmt.Println("  Tray:    Disabled (D-Bus session bus not available)")
		} else {
			app.trayMgr = tray.NewManager(
				cfg,
				app.currentConfigPath,
				func() (string, error) {
					return app.LoadConfig(app.currentConfigPath)
				},
				func() {
					fmt.Println("\nQuit requested from system tray. Exiting...")
					cancel()
					_ = audio.Terminate()
					os.Exit(0)
				},
			)
			if err := app.trayMgr.Start(); err != nil {
				log.Printf("⚠️  [TRAY] Failed to initialize system tray: %v (continuing without tray)", err)
				app.trayMgr = nil
			} else {
				fmt.Println("  Tray:    Active (StatusNotifierItem registered on D-Bus)")
			}
		}
	} else {
		fmt.Println("  Tray:    Disabled")
	}

	// Signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutdown signal received. Exiting...")
		cancel()
		_ = audio.Terminate()
		if app.trayMgr != nil {
			app.trayMgr.Quit()
		}

		os.Exit(0)
	}()

	hupChan := make(chan os.Signal, 1)
	signal.Notify(hupChan, syscall.SIGHUP)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-hupChan:
				fmt.Println("\nSIGHUP received. Reloading config...")
				if _, err := app.LoadConfig(app.currentConfigPath); err != nil {
					log.Printf("Error reloading config on SIGHUP: %v", err)
				}
			}
		}
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
	app.ipcServer.SetLoadHandler(app.LoadConfig)
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

	if app.trayMgr != nil {
		app.trayMgr.Quit()
	}
}

func (a *App) LoadConfig(path string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	targetPath := strings.TrimSpace(path)
	isFallback := false
	if targetPath == "" {
		isFallback = true
		targetPath = a.fallbackConfigPath
	}

	newCfg, err := config.Load(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to load config %q: %w", targetPath, err)
	}

	// Update STT engine if STT parameters changed
	sttChanged := a.cfg != nil && (newCfg.STT.BinaryPath != a.cfg.STT.BinaryPath ||
		newCfg.STT.ModelPath != a.cfg.STT.ModelPath ||
		newCfg.STT.Language != a.cfg.STT.Language ||
		newCfg.STT.Threads != a.cfg.STT.Threads)

	if a.sttEngine != nil && sttChanged {
		engine, err := stt.NewEngine(
			newCfg.STT.BinaryPath,
			newCfg.STT.ModelPath,
			newCfg.STT.Language,
			newCfg.STT.Threads,
		)
		if err != nil {
			return "", fmt.Errorf("failed to initialize STT engine with new config: %w", err)
		}
		a.sttEngine = engine
	}

	// Update hotkey listener combo if key changed
	if a.listener != nil && (a.cfg == nil || newCfg.Hotkey.Key != a.cfg.Hotkey.Key) {
		if err := a.listener.UpdateCombo(newCfg.Hotkey.Key); err != nil {
			log.Printf("Warning: failed to update hotkey combo to %q: %v", newCfg.Hotkey.Key, err)
		} else {
			fmt.Printf("🔄 [HOTKEY UPDATED] Now listening on %q\n", newCfg.Hotkey.Key)
		}
	}

	oldPath := a.currentConfigPath
	a.cfg = newCfg
	a.currentConfigPath = newCfg.ConfigPath

	if a.trayMgr != nil {
		a.trayMgr.UpdateConfig(newCfg, a.currentConfigPath)
	}

	var msg string
	if isFallback {
		msg = fmt.Sprintf("loaded fallback config from %s (%d commands)", a.currentConfigPath, len(newCfg.Commands))
	} else {
		msg = fmt.Sprintf("loaded config from %s (%d commands)", a.currentConfigPath, len(newCfg.Commands))
	}

	fmt.Printf("🔄 [CONFIG RELOADED] %s (previous: %s)\n", msg, oldPath)
	return msg, nil
}

func (a *App) OnPress() {
	a.mu.Lock()
	if a.isHandling || a.recorder.IsRecording() {
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()

	if a.trayMgr != nil {
		a.trayMgr.SetState(tray.StateRecording)
	}

	fmt.Println("\n🔴 [RECORDING] Combination pressed. Listening...")
	if err := a.recorder.Start(); err != nil {
		log.Printf("Failed to start recording: %v", err)
		if a.trayMgr != nil {
			a.trayMgr.SetState(tray.StateIdle)
		}
	}
}

func (a *App) OnRelease() {
	if !a.recorder.IsRecording() {
		return
	}

	samples, dur, err := a.recorder.Stop()
	if err != nil {
		log.Printf("Error stopping recording: %v", err)
		if a.trayMgr != nil {
			a.trayMgr.SetState(tray.StateIdle)
		}
		return
	}

	if dur < 300*time.Millisecond {
		fmt.Printf(
			"⚠️  [CANCELLED] Key held for only %.2fs (<0.3s), ignoring tap.\n",
			dur.Seconds(),
		)
		if a.trayMgr != nil {
			a.trayMgr.SetState(tray.StateIdle)
		}
		return
	}

	fmt.Printf(
		"⏹️  [RECORDED] Recorded %.2f seconds (%d samples). Transcribing...\n",
		dur.Seconds(),
		len(samples),
	)

	if a.trayMgr != nil {
		a.trayMgr.SetState(tray.StateTranscribing)
	}

	a.mu.Lock()
	a.isHandling = true
	cfg := a.cfg
	engine := a.sttEngine
	a.mu.Unlock()

	go func() {
		defer func() {
			a.mu.Lock()
			a.isHandling = false
			a.mu.Unlock()
			if a.trayMgr != nil {
				a.trayMgr.SetState(tray.StateIdle)
			}
		}()

		processRecordedAudio(cfg, engine, samples)
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
