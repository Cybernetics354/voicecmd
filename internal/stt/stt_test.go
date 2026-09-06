package stt

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestEngineTranscribe(t *testing.T) {
	wavFile := "../../recording.wav"
	if _, err := os.Stat(wavFile); err != nil {
		t.Skip("skipping integration test; recording.wav not found")
	}

	engine, err := NewEngine(
		"../../whisper.cpp/build/bin/whisper-cli",
		"../../whisper.cpp/models/ggml-base.en.bin",
		"en",
		2,
	)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := engine.Transcribe(ctx, wavFile)
	if err != nil {
		t.Fatalf("failed to transcribe: %v", err)
	}

	if result.CleanText == "" {
		t.Logf("Raw transcript: %q", result.RawText)
	} else {
		t.Logf("Clean transcript: %q (raw: %q)", result.CleanText, result.RawText)
	}
}
