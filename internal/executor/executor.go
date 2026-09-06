package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"voicecmd/internal/config"
)

// Result contains execution details.
type Result struct {
	Program string
	Args    []string
	Stdout  string
	Stderr  string
	Err     error
}

// Execute runs the specified command with arguments.
// If {transcript} is present in any argument, it is replaced with the transcribed text.
func Execute(ctx context.Context, cmdConfig config.CommandConfig, transcript string) *Result {
	res := &Result{
		Program: cmdConfig.Command.Program,
	}

	if res.Program == "" {
		res.Err = fmt.Errorf("no program specified in command configuration")
		return res
	}

	// Expand placeholders in arguments
	expandedArgs := make([]string, len(cmdConfig.Command.Args))
	for i, arg := range cmdConfig.Command.Args {
		expandedArgs[i] = strings.ReplaceAll(arg, "{transcript}", transcript)
	}
	res.Args = expandedArgs

	// Execute with timeout if context has no deadline
	execCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		execCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(execCtx, res.Program, res.Args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	res.Err = cmd.Run()
	res.Stdout = strings.TrimSpace(stdout.String())
	res.Stderr = strings.TrimSpace(stderr.String())

	return res
}
