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
}
