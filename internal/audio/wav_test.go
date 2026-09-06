package audio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteWAV(t *testing.T) {
	tmpDir := t.TempDir()
	wavFile := filepath.Join(tmpDir, "test.wav")

	// 1 second of 440Hz sine wave or zeros
	samples := make([]float32, 16000)
	for i := range samples {
		samples[i] = 0.5
	}

	err := WriteWAV(wavFile, samples, 16000, 1)
	if err != nil {
		t.Fatalf("WriteWAV failed: %v", err)
	}

	info, err := os.Stat(wavFile)
	if err != nil {
		t.Fatalf("os.Stat failed: %v", err)
	}

	expectedSize := 44 + 16000*2
	if info.Size() != int64(expectedSize) {
		t.Errorf("expected file size %d, got %d", expectedSize, info.Size())
	}
}
