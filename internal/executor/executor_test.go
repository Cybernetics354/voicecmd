package executor

import (
	"context"
	"testing"
	"time"
	"voicecmd/internal/config"
)

func TestExecute(t *testing.T) {
	cmdConfig := config.CommandConfig{
		Command: config.ExecCommandConfig{
			Program: "echo",
			Args:    []string{"heard:", "{transcript}"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	res := Execute(ctx, cmdConfig, "hello world")
	if res.Err != nil {
		t.Fatalf("Execute failed: %v", res.Err)
	}

	expectedStdout := "heard: hello world"
	if res.Stdout != expectedStdout {
		t.Errorf("expected stdout %q, got %q", expectedStdout, res.Stdout)
	}
}
