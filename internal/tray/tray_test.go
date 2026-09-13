package tray

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"voicecmd/internal/config"
)

func TestIconGeneration(t *testing.T) {
	states := []State{StateIdle, StateRecording, StateTranscribing}

	for _, s := range states {
		data := IconForState(s)
		if len(data) == 0 {
			t.Fatalf("IconForState(%s) returned empty byte slice", s)
		}

		// Verify PNG signature (89 50 4E 47 0D 0A 1A 0A)
		pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
		if !bytes.HasPrefix(data, pngHeader) {
			t.Errorf("IconForState(%s) does not have valid PNG header", s)
		}

		// Decode image config
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("Failed to decode PNG for state %s: %v", s, err)
		}

		bounds := img.Bounds()
		if bounds.Dx() != 64 || bounds.Dy() != 64 {
			t.Errorf("Expected 64x64 image for state %s, got %dx%d", s, bounds.Dx(), bounds.Dy())
		}
	}
}

func TestStateMethods(t *testing.T) {
	if StateIdle.String() != "idle" {
		t.Errorf("expected 'idle', got %q", StateIdle.String())
	}
	if StateRecording.String() != "recording" {
		t.Errorf("expected 'recording', got %q", StateRecording.String())
	}
	if StateTranscribing.String() != "transcribing" {
		t.Errorf("expected 'transcribing', got %q", StateTranscribing.String())
	}

	if StateIdle.StatusText() != "● Idle (Ready)" {
		t.Errorf("unexpected StatusText: %q", StateIdle.StatusText())
	}
	if StateRecording.StatusText() != "● Recording..." {
		t.Errorf("unexpected StatusText: %q", StateRecording.StatusText())
	}
	if StateTranscribing.StatusText() != "⏳ Transcribing..." {
		t.Errorf("unexpected StatusText: %q", StateTranscribing.StatusText())
	}

	idleTip := StateIdle.Tooltip("ctrl+space")
	if idleTip != "VoiceCmd - Idle (Hotkey: ctrl+space)" {
		t.Errorf("unexpected tooltip: %q", idleTip)
	}

	recTip := StateRecording.Tooltip("ctrl+space")
	if recTip != "VoiceCmd - Recording microphone..." {
		t.Errorf("unexpected tooltip: %q", recTip)
	}

	transTip := StateTranscribing.Tooltip("ctrl+space")
	if transTip != "VoiceCmd - Transcribing speech..." {
		t.Errorf("unexpected tooltip: %q", transTip)
	}
}

func TestFormattingHelpers(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hotkey.Key = "alt+v"
	cfg.STT.ModelPath = "/models/ggml-base.en.bin"
	cfg.STT.Language = "en"
	cfg.STT.Threads = 8
	cfg.STT.Threshold = 0.85
	cfg.Audio.Device = "PulseAudio Direct"
	cfg.Audio.SampleRate = 16000
	cfg.Commands["terminal"] = config.CommandConfig{
		Phrases: []string{"open terminal", "launch terminal"},
		Command: config.ExecCommandConfig{Program: "alacritty"},
	}

	if got := formatHotkey(cfg); got != "Hotkey: alt+v" {
		t.Errorf("unexpected formatHotkey: %q", got)
	}
	if got := formatSTT(cfg); got != "STT Model: ggml-base.en.bin (en)" {
		t.Errorf("unexpected formatSTT: %q", got)
	}
	if got := formatSTTDetails(cfg); got != "STT Engine: 8 threads | 0.85 thresh" {
		t.Errorf("unexpected formatSTTDetails: %q", got)
	}
	if got := formatAudio(cfg); got != "Audio: PulseAudio Direct (16000Hz)" {
		t.Errorf("unexpected formatAudio: %q", got)
	}
	if got := formatCommandsTitle(cfg); got != "Commands: 1 loaded" {
		t.Errorf("unexpected formatCommandsTitle: %q", got)
	}

	// Test nil config safety
	if got := formatHotkey(nil); got != "Hotkey: ctrl+space" {
		t.Errorf("unexpected formatHotkey(nil): %q", got)
	}
	if got := formatSTT(nil); got != "STT: whisper.cpp" {
		t.Errorf("unexpected formatSTT(nil): %q", got)
	}
	if got := formatSTTDetails(nil); got != "STT Engine: 4 threads" {
		t.Errorf("unexpected formatSTTDetails(nil): %q", got)
	}
	if got := formatAudio(nil); got != "Audio: 16000Hz (mono)" {
		t.Errorf("unexpected formatAudio(nil): %q", got)
	}
	if got := formatCommandsTitle(nil); got != "Commands: 0 loaded" {
		t.Errorf("unexpected formatCommandsTitle(nil): %q", got)
	}
}

func TestFormatConfigPath(t *testing.T) {
	if got := formatConfigPath(""); got != "Config: (default)" {
		t.Errorf("unexpected formatConfigPath(\"\"): %q", got)
	}

	home, _ := os.UserHomeDir()
	if home != "" {
		testPath := filepath.Join(home, ".config/voicecmd/config.yaml")
		expected := "Config: ~/.config/voicecmd/config.yaml"
		if got := formatConfigPath(testPath); got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	}
}

func TestOpenConfigFile(t *testing.T) {
	// Empty path
	if err := OpenConfigFile(""); err == nil {
		t.Errorf("expected error for empty path")
	}

	// Non-existent path
	if err := OpenConfigFile("/nonexistent/path/config.yaml"); err == nil {
		t.Errorf("expected error for nonexistent file")
	}
}

func TestManagerCreationAndUpdate(t *testing.T) {
	cfg := config.DefaultConfig()
	mgr := NewManager(cfg, "config/config.yaml", nil, nil)
	if mgr == nil {
		t.Fatalf("expected non-nil manager")
	}

	// Safe to call SetState and UpdateConfig before Start()
	mgr.SetState(StateRecording)
	if mgr.state != StateRecording {
		t.Errorf("expected state %s, got %s", StateRecording, mgr.state)
	}

	newCfg := config.DefaultConfig()
	newCfg.Hotkey.Key = "f9"
	mgr.UpdateConfig(newCfg, "new/config.yaml")
	if mgr.cfg.Hotkey.Key != "f9" {
		t.Errorf("expected updated hotkey 'f9', got %q", mgr.cfg.Hotkey.Key)
	}
	if mgr.configPath != "new/config.yaml" {
		t.Errorf("expected updated configPath 'new/config.yaml', got %q", mgr.configPath)
	}
}
