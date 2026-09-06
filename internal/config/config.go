package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Hotkey   HotkeyConfig             `yaml:"hotkey"`
	Hotket   HotkeyConfig             `yaml:"hotket,omitempty"` // Backwards compatibility with typo
	Audio    AudioConfig              `yaml:"audio"`
	STT      STTConfig                `yaml:"stt"`
	Commands map[string]CommandConfig `yaml:"commands"`
	Notify   ExecCommandConfig        `yaml:"notify"`
}

type HotkeyConfig struct {
	Key    string `yaml:"key"`    // Combination key (e.g. "ctrl+space", "alt+v", "KEY_LEFTCTRL+KEY_SPACE")
	Device string `yaml:"device"` // Optional specific input device path (e.g. "/dev/input/event6")
}

type AudioConfig struct {
	Device     string `yaml:"device"`      // Audio input device name (empty for default)
	SampleRate int    `yaml:"sample_rate"` // Sample rate (default: 16000)
	Channels   int    `yaml:"channels"`    // Channels count (default: 1 mono)
}

type STTConfig struct {
	BinaryPath string  `yaml:"binary_path"` // Path to whisper-cli binary
	ModelPath  string  `yaml:"model_path"`  // Path to ggml model file
	Language   string  `yaml:"language"`    // Spoken language (default: "en")
	Threads    int     `yaml:"threads"`     // Inference threads
	Threshold  float64 `yaml:"threshold"`   // Matching similarity threshold (0.0 - 1.0)
	KeepWav    bool    `yaml:"keep_wav"`    // Keep recording wav file after transcription
	WavPath    string  `yaml:"wav_path"`    // Path to save recording WAV
}

type CommandConfig struct {
	Phrases []string          `yaml:"phrases"`
	Command ExecCommandConfig `yaml:"command"`
	Notify  ExecCommandConfig `yaml:"notify"`
}

type ExecCommandConfig struct {
	Program string   `yaml:"program"`
	Args    []string `yaml:"args"`
	detach  bool     `yaml:"detach"`
}

// DefaultConfig returns a configuration with sensible default values.
func DefaultConfig() *Config {
	return &Config{
		Hotkey: HotkeyConfig{
			Key: "ctrl+space",
		},
		Audio: AudioConfig{
			SampleRate: 16000,
			Channels:   1,
		},
		STT: STTConfig{
			BinaryPath: "./whisper.cpp/build/bin/whisper-cli",
			ModelPath:  "./whisper.cpp/models/ggml-base.en.bin",
			Language:   "en",
			Threads:    4,
			Threshold:  0.75,
			KeepWav:    false,
			WavPath:    "/tmp/voicecmd_recording.wav",
		},
		Commands: make(map[string]CommandConfig),
		Notify: ExecCommandConfig{
			detach:  false,
			Program: "notify-send",
			Args:    []string{"VoiceCmd", "{transcript}"},
		},
	}
}

// Load loads configuration from the given file path.
// If path is empty, it attempts to find config in default locations.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path == "" {
		candidates := []string{
			"config/config.yaml",
			"config.yaml",
			filepath.Join(os.Getenv("HOME"), ".config/voicecmd/config.yaml"),
			"/etc/voicecmd/config.yaml",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				path = c
				break
			}
		}
	}

	if path == "" {
		return nil, fmt.Errorf(
			"no configuration file specified and none found in default locations",
		)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %q: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %q: %w", path, err)
	}

	// Handle backwards compatibility typo "hotket"
	if cfg.Hotkey.Key == "" && cfg.Hotket.Key != "" {
		cfg.Hotkey = cfg.Hotket
	}

	cfg.applyDefaults()
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Hotkey.Key == "" {
		c.Hotkey.Key = "ctrl+space"
	}
	if c.Audio.SampleRate <= 0 {
		c.Audio.SampleRate = 16000
	}
	if c.Audio.Channels <= 0 {
		c.Audio.Channels = 1
	}
	if c.STT.BinaryPath == "" {
		c.STT.BinaryPath = "./whisper.cpp/build/bin/whisper-cli"
	}
	if c.STT.ModelPath == "" {
		c.STT.ModelPath = "./whisper.cpp/models/ggml-base.en.bin"
	}
	if c.STT.Language == "" {
		c.STT.Language = "en"
	}
	if c.STT.Threads <= 0 {
		c.STT.Threads = 4
	}
	if c.STT.Threshold <= 0 {
		c.STT.Threshold = 0.75
	}
	if c.STT.WavPath == "" {
		c.STT.WavPath = "/tmp/voicecmd_recording.wav"
	}
}
