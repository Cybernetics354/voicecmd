package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Hotkey.Key != "ctrl+space" {
		t.Fatalf("expected default key 'ctrl+space', got %q", cfg.Hotkey.Key)
	}
	if cfg.Audio.SampleRate != 16000 {
		t.Fatalf("expected sample rate 16000, got %d", cfg.Audio.SampleRate)
	}
}

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `
hotkey:
  key: "alt+v"
audio:
  sample_rate: 16000
  channels: 1
stt:
  binary_path: "/usr/bin/whisper-cli"
  model_path: "/models/base.bin"
  threshold: 0.8
commands:
  browser:
    phrases:
      - "open browser"
      - "firefox"
    command:
      program: "firefox"
      args: ["--new-tab"]
`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Hotkey.Key != "alt+v" {
		t.Errorf("expected hotkey 'alt+v', got %q", cfg.Hotkey.Key)
	}
	if cfg.STT.Threshold != 0.8 {
		t.Errorf("expected threshold 0.8, got %f", cfg.STT.Threshold)
	}
	cmd, ok := cfg.Commands["browser"]
	if !ok {
		t.Fatalf("expected command 'browser' to exist")
	}
	if len(cmd.Phrases) != 2 {
		t.Errorf("expected 2 phrases, got %d", len(cmd.Phrases))
	}
	if cmd.Command.Program != "firefox" {
		t.Errorf("expected program 'firefox', got %q", cmd.Command.Program)
	}
}
