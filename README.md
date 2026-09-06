# VoiceCmd

A fast, lightweight, offline voice command daemon written in Go for Linux. 

Hold down a configured key combination (e.g., `Ctrl+Space`) to record your microphone. When you release the keys, [whisper.cpp](https://github.com/ggerganov/whisper.cpp) automatically transcribes your speech locally and executes the matched command according to your YAML configuration mapping.

---

## Features

- **Push-To-Talk Global Hotkey**: Uses Linux kernel `evdev` to capture key press and release events directly across X11, Wayland, and console/TTY.
- **Local & Offline Speech-to-Text**: Integrates directly with `whisper.cpp` (`whisper-cli`), ensuring low latency and privacy without cloud dependencies.
- **Intelligent Command Matching**:
  - Exact phrase matching
  - Substring & conversational matching (e.g. "hey computer open terminal")
  - Fuzzy Levenshtein edit distance & token overlap matching (handles spelling differences, background noise, or slight mispronunciations)
- **Dynamic Argument Expansion**: Use `{transcript}` in command arguments to pass the transcribed phrase to scripts or notifications.
- **Multiple Operational Modes**:
  - **Daemon mode** (`voicecmd`): Background hotkey listener with IPC socket support.
  - **Interactive Terminal PTT** (`voicecmd -ptt`): Press Enter to record, speak, and press Enter to stop. Great for testing without root/evdev permissions.
  - **Compositor / IPC Triggers** (`voicecmd -trigger [start|stop|status]`): Integrate directly with Wayland compositors like Niri, Sway, or Hyprland via Unix domain socket (`/tmp/voicecmd.sock`).
  - **Direct Transcription Test** (`voicecmd -transcribe file.wav`): Test whisper transcription and command matching on pre-recorded audio.
  - **Device Lister** (`voicecmd -list-devices`): View all microphone inputs and keyboard devices.
- **Systemd Integration**: Includes user systemd service unit.

---

## Architecture

```
voicecmd/
├── cmd/
│   └── voicecmd/
│       └── main.go           # CLI entry point, argument parsing, modes (daemon, ptt, trigger, transcribe)
├── config/
│   └── config.yaml           # Default configuration file with commands & keybindings
├── internal/
│   ├── audio/
│   │   ├── recorder.go       # PortAudio push-to-talk recorder
│   │   ├── wav.go            # 16-bit PCM 16kHz WAV encoder
│   │   └── silence_alsa.go   # ALSA error handler suppression
│   ├── config/
│   │   ├── config.go         # Configuration loader & defaults
│   │   └── config_test.go    # Config tests
│   ├── executor/
│   │   ├── executor.go       # Command execution with placeholder expansion
│   │   └── executor_test.go  # Executor tests
│   ├── hotkey/
│   │   ├── hotkey.go         # Evdev multi-keyboard listener (down/up state machine)
│   │   ├── keymap.go         # Key combo parser ("ctrl+space", "alt+v", "f9", etc.)
│   │   ├── ipc.go            # Unix domain socket server and client (/tmp/voicecmd.sock)
│   │   └── *_test.go         # Hotkey and IPC tests
│   ├── matcher/
│   │   ├── matcher.go        # Hybrid exact, token, and fuzzy similarity matcher
│   │   └── matcher_test.go   # Matcher tests
│   └── stt/
│       ├── stt.go            # Whisper STT engine
│       ├── cleaner.go        # Punctuation & whisper artifact cleaner
│       └── *_test.go         # STT tests
├── systemd/
│   ├── voicecmd.service      # Systemd user service unit
│   └── README.md             # Systemd installation instructions
└── Makefile                  # Build and management tasks
```

---

## Quick Start

### 1. Build

```bash
make build
```

This compiles the binary to `./voicecmd`.

### 2. Test Audio Devices

Verify your microphone and keyboard devices:

```bash
make list
# Or: ./voicecmd -list-devices
```

### 3. Test Interactive Push-To-Talk in Terminal

No special permissions required:

```bash
make ptt
# Or: ./voicecmd -ptt
```
Press **Enter**, say a command (e.g., *"open terminal"* or *"mute"*), and press **Enter** again.

---

## Keyboard Hotkey Permissions (for Evdev Daemon)

To allow `voicecmd` to monitor global keyboard hotkeys directly through the Linux `/dev/input/` subsystem:

1. Add your user account to the `input` group:
   ```bash
   sudo usermod -aG input $USER
   ```
2. Log out and log back in (or restart) for the group membership to take effect.
3. Verify your groups:
   ```bash
   groups
   ```
   You should see `input` in the list.

Now you can run the background daemon:
```bash
./voicecmd
```

---

## Configuration (`config/config.yaml`)

```yaml
hotkey:
  # Combination to hold down while speaking
  # Examples: "ctrl+space", "alt+v", "super+v", "f9", "KEY_LEFTCTRL+KEY_SPACE"
  key: "ctrl+space"
  # Optional: path to specific /dev/input/event* device, or empty to listen on all keyboards
  device: ""

audio:
  # Audio input device (empty string uses system default microphone)
  device: ""
  # Whisper requires 16000Hz mono audio
  sample_rate: 16000
  channels: 1

stt:
  # Path to whisper-cli binary
  binary_path: "./whisper.cpp/build/bin/whisper-cli"
  # Path to GGML model file
  model_path: "./whisper.cpp/models/ggml-base.en.bin"
  # Language ("en" or "auto")
  language: "en"
  # CPU threads for whisper inference
  threads: 4
  # Minimum similarity threshold (0.0 - 1.0) to match a command
  threshold: 0.75
  # Keep recorded WAV file after transcription (useful for debugging)
  keep_wav: false
  # WAV output path
  wav_path: "/tmp/voicecmd_recording.wav"

commands:
  terminal:
    phrases:
      - "open terminal"
      - "launch terminal"
      - "terminal"
    command:
      program: "alacritty"
      args: []

  browser:
    phrases:
      - "open browser"
      - "open brave"
      - "launch browser"
    command:
      program: "brave"
      args: []

  volume_up:
    phrases:
      - "volume up"
      - "turn up volume"
      - "louder"
    command:
      program: "wpctl"
      args: ["set-volume", "@DEFAULT_AUDIO_SINK@", "5%+"]

  volume_down:
    phrases:
      - "volume down"
      - "turn down volume"
      - "quieter"
    command:
      program: "wpctl"
      args: ["set-volume", "@DEFAULT_AUDIO_SINK@", "5%-"]

  volume_mute:
    phrases:
      - "mute audio"
      - "mute sound"
      - "mute"
      - "unmute"
    command:
      program: "wpctl"
      args: ["set-mute", "@DEFAULT_AUDIO_SINK@", "toggle"]

  notification:
    phrases:
      - "hello"
      - "say hello"
      - "test voice"
    command:
      program: "notify-send"
      args: ["VoiceCmd", "Heard voice command: {transcript}"]

  shutdown:
    phrases:
      - "shut down"
      - "turn off computer"
      - "power off"
    command:
      program: "systemctl"
      args: ["poweroff"]
```

---

## Wayland / Compositor Integration (Niri, Sway, Hyprland)

If you use a Wayland compositor and prefer binding the hotkey inside your compositor config instead of evdev, you can trigger `voicecmd` via its IPC socket:

### Niri (`~/.config/niri/cfg/keybinds.kdl`)
```kdl
// Trigger recording on Mod+V press
binds {
    Mod+V { spawn "voicecmd" "-trigger" "start"; }
    // Or toggle mode:
    Mod+Shift+V { spawn "voicecmd" "-trigger" "stop"; }
}
```

### Hyprland (`~/.config/hypr/hyprland.conf`)
```ini
bind = SUPER, V, exec, voicecmd -trigger start
bindr = SUPER, V, exec, voicecmd -trigger stop
```

---

## Systemd Service

To run `voicecmd` automatically on login as a systemd user service:

```bash
# 1. Install binary and default configuration
make install

# 2. Copy and enable user service unit
mkdir -p ~/.config/systemd/user
cp systemd/voicecmd.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now voicecmd.service

# 3. View live status
systemctl --user status voicecmd.service
```

---

## Testing

Run all unit and integration tests:

```bash
make test
```
