// Package dictate implements hold-to-dictate: record while key held, transcribe on release, type result.
package dictate

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"voicecmd/internal/audio"
	"voicecmd/internal/config"
	"voicecmd/internal/stt"
)

// TypeText types text into the focused window using the configured typer program.
// Supports "xdotool" and "ydotool".
func TypeText(typer, text string) error {
	if text == "" {
		return nil
	}
	var cmd *exec.Cmd
	switch typer {
	case "ydotool":
		cmd = exec.Command("ydotool", "type", "--", text)
	default: // xdotool
		// --clearmodifiers: release held modifier keys so they don't corrupt output
		cmd = exec.Command("xdotool", "type", "--clearmodifiers", "--", text)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s type failed: %w (output: %s)", typer, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Handle is called when the dictate key is released.
// It stops the recorder, transcribes, and types the result.
func Handle(cfg *config.Config, engine *stt.Engine, rec *audio.Recorder) {
	if !rec.IsRecording() {
		return
	}

	samples, dur, err := rec.Stop()
	if err != nil {
		log.Printf("[DICTATE] Error stopping recording: %v", err)
		return
	}

	if dur < 300*time.Millisecond {
		fmt.Printf("[DICTATE] Too short (%.2fs), ignored.\n", dur.Seconds())
		return
	}

	fmt.Printf("[DICTATE] Recorded %.2fs. Transcribing...\n", dur.Seconds())

	wavPath := cfg.STT.WavPath
	if wavPath == "" {
		wavPath = "/tmp/voicecmd_recording.wav"
	}
	// Use separate wav so dictate and command recording don't collide
	dictateWav := wavPath[:len(wavPath)-len(".wav")] + "_dictate.wav"
	if !strings.HasSuffix(wavPath, ".wav") {
		dictateWav = wavPath + "_dictate"
	}

	if err := audio.WriteWAV(dictateWav, samples, cfg.Audio.SampleRate, cfg.Audio.Channels); err != nil {
		log.Printf("[DICTATE] Failed to write WAV: %v", err)
		return
	}
	if !cfg.STT.KeepWav {
		defer func() { _ = exec.Command("rm", "-f", dictateWav).Run() }()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	res, err := engine.Transcribe(ctx, dictateWav)
	if err != nil {
		log.Printf("[DICTATE] Transcription error: %v", err)
		return
	}

	fmt.Printf("[DICTATE] Transcribed %q (took %v)\n", res.CleanText, time.Since(start))

	if res.CleanText == "" {
		fmt.Println("[DICTATE] No speech detected.")
		return
	}

	// Append a space after the typed text for natural flow
	typed := res.CleanText + " "
	if err := TypeText(cfg.Dictate.Typer, typed); err != nil {
		log.Printf("[DICTATE] Type error: %v", err)
	} else {
		fmt.Printf("[DICTATE] Typed %q\n", typed)
	}
}
