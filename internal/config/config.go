package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Hotkey     HotkeyConfig             `yaml:"hotkey"`
	Hotket     HotkeyConfig             `yaml:"hotket,omitempty"` // Backwards compatibility with typo
	Audio      AudioConfig              `yaml:"audio"`
	STT        STTConfig                `yaml:"stt"`
	Dictate    DictateConfig            `yaml:"dictate"`
	Commands   map[string]CommandConfig `yaml:"commands"`
	Notify     ExecCommandConfig        `yaml:"notify"`
	Tray       TrayConfig               `yaml:"tray"`
	ConfigPath string                   `yaml:"-"`
}

// DictateConfig controls the hold-to-dictate feature.
type DictateConfig struct {
	Enabled bool   `yaml:"enabled"` // Enable dictate mode (default: false)
	Key     string `yaml:"key"`     // Hotkey combo to hold while speaking (e.g. "alt+space")
	Device  string `yaml:"device"`  // Optional specific input device path
	Typer   string `yaml:"typer"`   // Program to type text: "xdotool" (default) or "ydotool"
}

type TrayConfig struct {
	Enabled *bool `yaml:"enabled"` // Enable system tray icon and menu (default: true)
}

// IsEnabled returns true if system tray is enabled (defaults to true if unset).
func (t TrayConfig) IsEnabled() bool {
	if t.Enabled == nil {
		return true
	}
	return *t.Enabled
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
		Tray: TrayConfig{
			Enabled: boolPtr(true),
		},
	}
}

func boolPtr(b bool) *bool {
	return &b
}

// ExpandPath expands environment variables and ~ in file paths.
func ExpandPath(path string) string {
	if path == "" {
		return ""
	}
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return home
		}
		return os.Getenv("HOME")
	}
	if strings.HasPrefix(path, "~/") {
		home := ""
		if h, err := os.UserHomeDir(); err == nil && h != "" {
			home = h
		} else {
			home = os.Getenv("HOME")
		}
		if home != "" {
			return filepath.Join(home, path[2:])
		}
	}
	return os.ExpandEnv(path)
}

// DefaultCandidates returns candidate paths searched for fallback config files.
func DefaultCandidates() []string {
	var candidates []string
	home := ""
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		home = h
	} else {
		home = os.Getenv("HOME")
	}

	candidates = append(candidates,
		"config/config.yaml",
		"config/config.yml",
		"config/config.conf",
		"config.yaml",
		"config.yml",
		"config.conf",
	)

	if home != "" {
		candidates = append(candidates,
			filepath.Join(home, ".config/voicecmd/config.yaml"),
			filepath.Join(home, ".config/voicecmd/config.yml"),
			filepath.Join(home, ".config/voicecmd/config.conf"),
		)
	}

	candidates = append(candidates,
		"/etc/voicecmd/config.yaml",
		"/etc/voicecmd/config.yml",
		"/etc/voicecmd/config.conf",
	)

	return candidates
}

// FindFallbackConfig searches the default candidate locations for an existing config file.
func FindFallbackConfig() (string, error) {
	for _, c := range DefaultCandidates() {
		if _, err := os.Stat(c); err == nil {
			if abs, err := filepath.Abs(c); err == nil {
				return abs, nil
			}
			return c, nil
		}
	}
	return "", fmt.Errorf("no configuration file specified and none found in default locations")
}

// Load loads configuration from the given file path.
// If path is empty, it attempts to find config in default fallback locations.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	resolvedPath := path
	if resolvedPath == "" {
		fallback, err := FindFallbackConfig()
		if err != nil {
			return nil, err
		}
		resolvedPath = fallback
	} else {
		resolvedPath = ExpandPath(resolvedPath)
		if abs, err := filepath.Abs(resolvedPath); err == nil {
			resolvedPath = abs
		}
	}

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %q: %w", resolvedPath, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %q: %w", resolvedPath, err)
	}

	// Handle backwards compatibility typo "hotket"
	if cfg.Hotkey.Key == "" && cfg.Hotket.Key != "" {
		cfg.Hotkey = cfg.Hotket
	}

	cfg.applyDefaults()
	cfg.ConfigPath = resolvedPath
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
	if c.Dictate.Enabled && c.Dictate.Key == "" {
		c.Dictate.Key = "alt+space"
	}
	if c.Dictate.Typer == "" {
		c.Dictate.Typer = "xdotool"
	}
	if c.Tray.Enabled == nil {
		c.Tray.Enabled = boolPtr(true)
	}
}
