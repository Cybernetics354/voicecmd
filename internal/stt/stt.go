package stt

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Engine wraps whisper.cpp CLI for transcribing speech.
type Engine struct {
	binaryPath string
	modelPath  string
	language   string
	threads    int
}

// Result contains raw and cleaned transcription results.
type Result struct {
	RawText   string
	CleanText string
}

// NewEngine creates and validates a new Whisper STT engine.
func NewEngine(binaryPath, modelPath, language string, threads int) (*Engine, error) {
	resolvedBinary, err := resolveBinary(binaryPath)
	if err != nil {
		return nil, err
	}

	resolvedModel, err := resolveFile(modelPath)
	if err != nil {
		return nil, fmt.Errorf("whisper model not found at %q: %w", modelPath, err)
	}

	if language == "" {
		language = "en"
	}
	if threads <= 0 {
		threads = 4
	}

	return &Engine{
		binaryPath: resolvedBinary,
		modelPath:  resolvedModel,
		language:   language,
		threads:    threads,
	}, nil
}

// Transcribe invokes whisper-cli on the given WAV file and returns the transcription.
func (e *Engine) Transcribe(ctx context.Context, wavPath string) (*Result, error) {
	if _, err := os.Stat(wavPath); err != nil {
		return nil, fmt.Errorf("wav file not found %q: %w", wavPath, err)
	}

	args := []string{
		"-m", e.modelPath,
		"-f", wavPath,
		"-l", e.language,
		"-t", strconv.Itoa(e.threads),
		"-np", // no prints
		"-nt", // no timestamps
	}

	cmd := exec.CommandContext(ctx, e.binaryPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("whisper-cli execution failed: %w (stderr: %s)", err, stderr.String())
	}

	raw := strings.TrimSpace(stdout.String())
	clean := CleanTranscript(raw)

	return &Result{
		RawText:   raw,
		CleanText: clean,
	}, nil
}

func resolveBinary(bin string) (string, error) {
	// 1. Direct path check
	if _, err := os.Stat(bin); err == nil {
		abs, err := filepath.Abs(bin)
		if err == nil {
			return abs, nil
		}
		return bin, nil
	}

	// 2. Look in PATH
	if looked, err := exec.LookPath(bin); err == nil {
		return looked, nil
	}

	// 3. Fallback check for common project paths
	commonPaths := []string{
		"./whisper.cpp/build/bin/whisper-cli",
		"../whisper.cpp/build/bin/whisper-cli",
		"./whisper.cpp/build/bin/main",
	}
	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs, nil
		}
	}

	return "", fmt.Errorf("whisper-cli binary not found at %q or in PATH", bin)
}

func resolveFile(path string) (string, error) {
	if _, err := os.Stat(path); err == nil {
		abs, err := filepath.Abs(path)
		if err == nil {
			return abs, nil
		}
		return path, nil
	}

	commonPaths := []string{
		"./whisper.cpp/models/ggml-base.en.bin",
		"../whisper.cpp/models/ggml-base.en.bin",
		"./whisper.cpp/models/ggml-base.bin",
	}
	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs, nil
		}
	}

	return "", fmt.Errorf("file not found: %s", path)
}
