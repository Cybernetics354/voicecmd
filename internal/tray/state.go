package tray

// State represents the current operational state of VoiceCmd.
type State string

const (
	// StateIdle indicates VoiceCmd is idle and ready, waiting for hotkey or trigger.
	StateIdle State = "idle"
	// StateRecording indicates VoiceCmd is actively recording audio from microphone.
	StateRecording State = "recording"
	// StateTranscribing indicates VoiceCmd is transcribing audio with Whisper and executing commands.
	StateTranscribing State = "transcribing"
	// StateDictating indicates VoiceCmd is recording audio for dictation (hold-to-dictate mode).
	StateDictating State = "dictating"
)

// String returns the string representation of State.
func (s State) String() string {
	return string(s)
}

// StatusText returns a human-readable title for the status menu item.
func (s State) StatusText() string {
	switch s {
	case StateRecording:
		return "● Recording..."
	case StateTranscribing:
		return "⏳ Transcribing..."
	case StateDictating:
		return "🎙 Dictating..."
	default:
		return "● Idle (Ready)"
	}
}

// Tooltip returns the tray icon hover tooltip corresponding to the state.
func (s State) Tooltip(hotkey string) string {
	switch s {
	case StateRecording:
		return "VoiceCmd - Recording microphone..."
	case StateTranscribing:
		return "VoiceCmd - Transcribing speech..."
	case StateDictating:
		return "VoiceCmd - Dictating (release to type)..."
	default:
		if hotkey != "" {
			return "VoiceCmd - Idle (Hotkey: " + hotkey + ")"
		}
		return "VoiceCmd - Idle"
	}
}
