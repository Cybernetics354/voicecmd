package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"voicecmd/internal/config"
	"voicecmd/internal/hotkey"
	"voicecmd/internal/tray"
)

func TestAppLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	fallbackFile := filepath.Join(tmpDir, "fallback.yaml")
	customFile := filepath.Join(tmpDir, "custom.conf")

	fallbackContent := `
hotkey:
  key: "ctrl+space"
commands:
  hello:
    phrases: ["hello"]
    command:
      program: "echo"
      args: ["hello"]
`
	customContent := `
hotkey:
  key: "alt+v"
commands:
  world:
    phrases: ["world"]
    command:
      program: "echo"
      args: ["world"]
  terminal:
    phrases: ["terminal"]
    command:
      program: "alacritty"
`
	if err := os.WriteFile(fallbackFile, []byte(fallbackContent), 0644); err != nil {
		t.Fatalf("failed to write fallbackFile: %v", err)
	}
	if err := os.WriteFile(customFile, []byte(customContent), 0644); err != nil {
		t.Fatalf("failed to write customFile: %v", err)
	}

	initialCfg, err := config.Load(fallbackFile)
	if err != nil {
		t.Fatalf("failed to load initial config: %v", err)
	}

	app := &App{
		cfg:                initialCfg,
		fallbackConfigPath: fallbackFile,
		currentConfigPath:  initialCfg.ConfigPath,
	}

	if _, ok := app.cfg.Commands["hello"]; !ok {
		t.Fatalf("expected initial config to have 'hello' command")
	}

	// 1. Load custom config file (e.g. .conf)
	msg, err := app.LoadConfig(customFile)
	if err != nil {
		t.Fatalf("LoadConfig(customFile) failed: %v", err)
	}
	t.Logf("LoadConfig(customFile) response: %s", msg)

	if _, ok := app.cfg.Commands["world"]; !ok {
		t.Errorf("expected 'world' command after loading custom config")
	}
	if _, ok := app.cfg.Commands["hello"]; ok {
		t.Errorf("did not expect 'hello' command in custom config")
	}
	if app.cfg.Hotkey.Key != "alt+v" {
		t.Errorf("expected hotkey 'alt+v', got %q", app.cfg.Hotkey.Key)
	}
	if app.currentConfigPath != customFile {
		t.Errorf("expected currentConfigPath %q, got %q", customFile, app.currentConfigPath)
	}

	// 2. Load fallback config (empty path)
	msg, err = app.LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig('') failed: %v", err)
	}
	t.Logf("LoadConfig('') response: %s", msg)

	if _, ok := app.cfg.Commands["hello"]; !ok {
		t.Errorf("expected 'hello' command after fallback reload")
	}
	if _, ok := app.cfg.Commands["world"]; ok {
		t.Errorf("did not expect 'world' command after fallback reload")
	}
	if app.cfg.Hotkey.Key != "ctrl+space" {
		t.Errorf("expected hotkey 'ctrl+space', got %q", app.cfg.Hotkey.Key)
	}
	if app.currentConfigPath != fallbackFile {
		t.Errorf("expected currentConfigPath %q, got %q", fallbackFile, app.currentConfigPath)
	}

	// 3. Error on non-existent config file
	_, err = app.LoadConfig("/nonexistent/file.yaml")
	if err == nil {
		t.Errorf("expected error when loading nonexistent file, got nil")
	}
	// Verify state wasn't corrupted
	if _, ok := app.cfg.Commands["hello"]; !ok {
		t.Errorf("expected 'hello' command to remain after failed load")
	}
}

func TestAppIPCIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	fallbackFile := filepath.Join(tmpDir, "fallback.yaml")
	customFile := filepath.Join(tmpDir, "custom.conf")
	sockPath := filepath.Join(tmpDir, "test_voicecmd.sock")

	fallbackContent := `
hotkey:
  key: "ctrl+space"
commands:
  fallback_cmd:
    phrases: ["fallback"]
    command:
      program: "echo"
      args: ["fallback"]
`
	customContent := `
hotkey:
  key: "super+v"
commands:
  custom_cmd1:
    phrases: ["custom one"]
    command:
      program: "echo"
      args: ["1"]
  custom_cmd2:
    phrases: ["custom two"]
    command:
      program: "echo"
      args: ["2"]
`
	if err := os.WriteFile(fallbackFile, []byte(fallbackContent), 0644); err != nil {
		t.Fatalf("failed to write fallbackFile: %v", err)
	}
	if err := os.WriteFile(customFile, []byte(customContent), 0644); err != nil {
		t.Fatalf("failed to write customFile: %v", err)
	}

	initialCfg, err := config.Load(fallbackFile)
	if err != nil {
		t.Fatalf("failed to load initial config: %v", err)
	}

	app := &App{
		cfg:                initialCfg,
		fallbackConfigPath: fallbackFile,
		currentConfigPath:  initialCfg.ConfigPath,
	}

	server := hotkey.NewIPCServer(sockPath, app.OnPress, app.OnRelease, func() string { return "idle" })
	server.SetLoadHandler(app.LoadConfig)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = server.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Test 1: Load custom config via IPC
	resp, err := hotkey.SendIPCCommand(sockPath, "load "+customFile)
	if err != nil {
		t.Fatalf("SendIPCCommand load custom failed: %v", err)
	}
	expectedCustomResp := "OK: loaded config from " + customFile + " (2 commands)"
	if resp != expectedCustomResp {
		t.Errorf("expected %q, got %q", expectedCustomResp, resp)
	}
	if _, ok := app.cfg.Commands["custom_cmd1"]; !ok {
		t.Errorf("expected custom_cmd1 to be present")
	}

	// Test 2: Load fallback config via IPC (no argument)
	resp, err = hotkey.SendIPCCommand(sockPath, "load")
	if err != nil {
		t.Fatalf("SendIPCCommand load fallback failed: %v", err)
	}
	expectedFallbackResp := "OK: loaded fallback config from " + fallbackFile + " (1 commands)"
	if resp != expectedFallbackResp {
		t.Errorf("expected %q, got %q", expectedFallbackResp, resp)
	}
	if _, ok := app.cfg.Commands["fallback_cmd"]; !ok {
		t.Errorf("expected fallback_cmd to be present")
	}

	// Test 3: Load invalid file via IPC
	resp, err = hotkey.SendIPCCommand(sockPath, "load /nonexistent/file.conf")
	if err != nil {
		t.Fatalf("SendIPCCommand load error failed: %v", err)
	}
	if !strings.HasPrefix(resp, "ERR:") {
		t.Errorf("expected ERR: prefix for nonexistent file, got %q", resp)
	}
}

func TestAppTrayIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	fallbackFile := filepath.Join(tmpDir, "fallback.yaml")
	content := `
hotkey:
  key: "ctrl+space"
tray:
  enabled: true
commands:
  hello:
    phrases: ["hello"]
    command:
      program: "echo"
`
	if err := os.WriteFile(fallbackFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write fallbackFile: %v", err)
	}

	cfg, err := config.Load(fallbackFile)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	reloadCalled := false
	quitCalled := false

	app := &App{
		cfg:                cfg,
		fallbackConfigPath: fallbackFile,
		currentConfigPath:  fallbackFile,
	}

	mgr := tray.NewManager(
		cfg,
		fallbackFile,
		func() (string, error) {
			reloadCalled = true
			return app.LoadConfig(fallbackFile)
		},
		func() {
			quitCalled = true
		},
	)
	app.trayMgr = mgr

	// Test state changes
	mgr.SetState(tray.StateRecording)
	mgr.SetState(tray.StateTranscribing)
	mgr.SetState(tray.StateIdle)

	// Test config reload propagation to tray
	newConfigFile := filepath.Join(tmpDir, "new.yaml")
	newContent := `
hotkey:
  key: "super+v"
commands:
  test:
    phrases: ["test phrase"]
    command:
      program: "notify-send"
`
	if err := os.WriteFile(newConfigFile, []byte(newContent), 0644); err != nil {
		t.Fatalf("failed to write newConfigFile: %v", err)
	}

	msg, err := app.LoadConfig(newConfigFile)
	if err != nil {
		t.Fatalf("app.LoadConfig failed: %v", err)
	}
	if !strings.Contains(msg, "new.yaml") {
		t.Errorf("unexpected load message: %q", msg)
	}

	_ = reloadCalled
	_ = quitCalled
}
