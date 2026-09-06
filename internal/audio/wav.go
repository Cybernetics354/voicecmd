package audio

import (
	"encoding/binary"
	"fmt"
	"os"
)

// WriteWAV writes 32-bit floating point audio samples to a standard 16-bit PCM WAV file.
func WriteWAV(filename string, samples []float32, sampleRate, channels int) error {
	const bitsPerSample = 16

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create wav file %q: %w", filename, err)
	}
	defer file.Close()

	dataSize := len(samples) * 2 // 2 bytes per sample (16-bit PCM)
	fileSize := 36 + dataSize

	// RIFF header
	if _, err := file.Write([]byte("RIFF")); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint32(fileSize)); err != nil {
		return err
	}
	if _, err := file.Write([]byte("WAVE")); err != nil {
		return err
	}

	// "fmt " chunk
	if _, err := file.Write([]byte("fmt ")); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint32(16)); err != nil { // chunk size
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint16(1)); err != nil { // PCM format
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint16(channels)); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint32(sampleRate)); err != nil {
		return err
	}

	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8

	if err := binary.Write(file, binary.LittleEndian, uint32(byteRate)); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint16(blockAlign)); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint16(bitsPerSample)); err != nil {
		return err
	}

	// "data" chunk
	if _, err := file.Write([]byte("data")); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint32(dataSize)); err != nil {
		return err
	}

	// Samples conversion and writing
	for _, sample := range samples {
		if sample > 1.0 {
			sample = 1.0
		} else if sample < -1.0 {
			sample = -1.0
		}
		val := int16(sample * 32767)
		if err := binary.Write(file, binary.LittleEndian, val); err != nil {
			return err
		}
	}

	return nil
}
