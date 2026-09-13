package hotkey

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestIPCServer(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "voicecmd_test.sock")

	var pressCount int32
	var releaseCount int32

	onPress := func() {
		atomic.AddInt32(&pressCount, 1)
	}
	onRelease := func() {
		atomic.AddInt32(&releaseCount, 1)
	}
	getStatus := func() string {
		return "testing"
	}

	server := NewIPCServer(sockPath, onPress, onRelease, getStatus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = server.Start(ctx)
	}()

	// Give socket time to bind
	time.Sleep(50 * time.Millisecond)

	// Send status
	resp, err := SendIPCCommand(sockPath, "status")
	if err != nil {
		t.Fatalf("SendIPCCommand status failed: %v", err)
	}
	if resp != "OK: status=testing" {
		t.Errorf("unexpected response: %q", resp)
	}

	// Send start
	resp, err = SendIPCCommand(sockPath, "start")
	if err != nil {
		t.Fatalf("SendIPCCommand start failed: %v", err)
	}
	if resp != "OK: recording started" {
		t.Errorf("unexpected response: %q", resp)
	}

	// Send stop
	resp, err = SendIPCCommand(sockPath, "stop")
	if err != nil {
		t.Fatalf("SendIPCCommand stop failed: %v", err)
	}
	if resp != "OK: recording stopped, transcribing" {
		t.Errorf("unexpected response: %q", resp)
	}

	if atomic.LoadInt32(&pressCount) != 1 {
		t.Errorf("expected 1 press event, got %d", pressCount)
	}
	if atomic.LoadInt32(&releaseCount) != 1 {
		t.Errorf("expected 1 release event, got %d", releaseCount)
	}

	// Test load command before setting handler
	resp, err = SendIPCCommand(sockPath, "load /tmp/config.yaml")
	if err != nil {
		t.Fatalf("SendIPCCommand load without handler failed: %v", err)
	}
	if resp != "ERR: config loading not supported" {
		t.Errorf("unexpected response: %q", resp)
	}

	// Set load handler
	var lastLoadedPath string
	server.SetLoadHandler(func(path string) (string, error) {
		lastLoadedPath = path
		if path == "" {
			return "loaded fallback config from /etc/voicecmd.yaml (3 commands)", nil
		}
		if path == "/nonexistent" {
			return "", &customTestError{msg: "file not found"}
		}
		return "loaded config from " + path + " (5 commands)", nil
	})

	// Test load with specified path
	resp, err = SendIPCCommand(sockPath, "load ~/.config/voicecmd/config.conf")
	if err != nil {
		t.Fatalf("SendIPCCommand load with path failed: %v", err)
	}
	if resp != "OK: loaded config from ~/.config/voicecmd/config.conf (5 commands)" {
		t.Errorf("unexpected response: %q", resp)
	}
	if lastLoadedPath != "~/.config/voicecmd/config.conf" {
		t.Errorf("expected lastLoadedPath to be ~/.config/voicecmd/config.conf, got %q", lastLoadedPath)
	}

	// Test load without specified path (fallback)
	resp, err = SendIPCCommand(sockPath, "load")
	if err != nil {
		t.Fatalf("SendIPCCommand load without path failed: %v", err)
	}
	if resp != "OK: loaded fallback config from /etc/voicecmd.yaml (3 commands)" {
		t.Errorf("unexpected response: %q", resp)
	}
	if lastLoadedPath != "" {
		t.Errorf("expected lastLoadedPath to be empty for fallback, got %q", lastLoadedPath)
	}

	// Test load error
	resp, err = SendIPCCommand(sockPath, "load /nonexistent")
	if err != nil {
		t.Fatalf("SendIPCCommand load error case failed: %v", err)
	}
	if resp != "ERR: file not found" {
		t.Errorf("unexpected error response: %q", resp)
	}

	// Test unknown command
	resp, err = SendIPCCommand(sockPath, "invalid_cmd")
	if err != nil {
		t.Fatalf("SendIPCCommand invalid_cmd failed: %v", err)
	}
	if resp != "ERR: unknown command \"invalid_cmd\". Valid commands: start, stop, status, load" {
		t.Errorf("unexpected unknown command response: %q", resp)
	}
}

type customTestError struct {
	msg string
}

func (e *customTestError) Error() string {
	return e.msg
}
