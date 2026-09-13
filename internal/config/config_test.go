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
	if cfg.ConfigPath != configFile {
		t.Errorf("expected ConfigPath %q, got %q", configFile, cfg.ConfigPath)
	}
}

func TestExpandPath(t *testing.T) {
	if got := ExpandPath(""); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}

	home, _ := os.UserHomeDir()
	if home != "" {
		if got := ExpandPath("~"); got != home {
			t.Errorf("expected %q, got %q", home, got)
		}
		expectedSub := filepath.Join(home, ".config/voicecmd/config.conf")
		if got := ExpandPath("~/.config/voicecmd/config.conf"); got != expectedSub {
			t.Errorf("expected %q, got %q", expectedSub, got)
		}
	}

	t.Setenv("VOICECMD_TEST_DIR", "/opt/voicecmd")
	if got := ExpandPath("$VOICECMD_TEST_DIR/custom.yaml"); got != "/opt/voicecmd/custom.yaml" {
		t.Errorf("expected /opt/voicecmd/custom.yaml, got %q", got)
	}
}

func TestLoadConfig_ConfExtension(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.conf")

	content := `
hotkey:
  key: "super+space"
commands:
  terminal:
    phrases:
      - "open terminal"
    command:
      program: "alacritty"
`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("failed to load .conf config: %v", err)
	}

	if cfg.Hotkey.Key != "super+space" {
		t.Errorf("expected hotkey 'super+space', got %q", cfg.Hotkey.Key)
	}
	if _, ok := cfg.Commands["terminal"]; !ok {
		t.Errorf("expected 'terminal' command to be loaded")
	}
	if cfg.ConfigPath != configFile {
		t.Errorf("expected ConfigPath %q, got %q", configFile, cfg.ConfigPath)
	}
}

func TestLoadConfig_Fallback(t *testing.T) {
	// Calling Load("") should find fallback candidate (such as config/config.yaml in repo)
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("failed to load fallback config: %v", err)
	}
	if cfg.ConfigPath == "" {
		t.Errorf("expected non-empty ConfigPath for fallback load")
	}
}

func TestTrayConfig(t *testing.T) {
	// Default config should have tray enabled
	def := DefaultConfig()
	if !def.Tray.IsEnabled() {
		t.Errorf("expected default tray to be enabled")
	}

	tmpDir := t.TempDir()
	// Test explicitly disabled tray
	disabledFile := filepath.Join(tmpDir, "disabled.yaml")
	if err := os.WriteFile(disabledFile, []byte("tray:\n  enabled: false\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	cfgDisabled, err := Load(disabledFile)
	if err != nil {
		t.Fatalf("failed to load disabled tray config: %v", err)
	}
	if cfgDisabled.Tray.IsEnabled() {
		t.Errorf("expected tray to be disabled")
	}

	// Test explicitly enabled tray
	enabledFile := filepath.Join(tmpDir, "enabled.yaml")
	if err := os.WriteFile(enabledFile, []byte("tray:\n  enabled: true\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	cfgEnabled, err := Load(enabledFile)
	if err != nil {
		t.Fatalf("failed to load enabled tray config: %v", err)
	}
	if !cfgEnabled.Tray.IsEnabled() {
		t.Errorf("expected tray to be enabled")
	}
}

