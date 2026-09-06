package matcher

import (
	"testing"
	"voicecmd/internal/config"
)

func TestMatcher(t *testing.T) {
	commands := map[string]config.CommandConfig{
		"shutdown": {
			Phrases: []string{"shut down", "turn off computer", "power off"},
			Command: config.ExecCommandConfig{
				Program: "systemctl",
				Args:    []string{"poweroff"},
			},
		},
		"terminal": {
			Phrases: []string{"open terminal", "launch terminal"},
			Command: config.ExecCommandConfig{
				Program: "alacritty",
				Args:    []string{},
			},
		},
		"volume_up": {
			Phrases: []string{"volume up", "turn up volume"},
			Command: config.ExecCommandConfig{
				Program: "wpctl",
				Args:    []string{"set-volume", "@DEFAULT_AUDIO_SINK@", "5%+"},
			},
		},
	}

	tests := []struct {
		transcript    string
		expectedCmd   string
		threshold     float64
		expectSuccess bool
	}{
		// Exact matches
		{"shut down", "shutdown", 0.75, true},
		{"Shut down.", "shutdown", 0.75, true},
		{"open terminal", "terminal", 0.75, true},

		// Substring / conversational matches
		{"please shut down", "shutdown", 0.75, true},
		{"hey computer open terminal now", "terminal", 0.70, true},

		// Fuzzy / typo matches
		{"shutdown", "shutdown", 0.75, true},      // 1 char edit distance
		{"volume up please", "volume_up", 0.75, true},
		{"power off", "shutdown", 0.75, true},

		// Non-matching
		{"play some jazz music", "", 0.75, false},
		{"random noise", "", 0.75, false},
		{"", "", 0.75, false},
	}

	for _, tc := range tests {
		result := Match(tc.transcript, commands, tc.threshold)
		if tc.expectSuccess {
			if result == nil {
				t.Errorf("Match(%q) expected match %q, got nil", tc.transcript, tc.expectedCmd)
			} else if result.CommandName != tc.expectedCmd {
				t.Errorf("Match(%q) expected command %q, got %q (score: %f)", tc.transcript, tc.expectedCmd, result.CommandName, result.Score)
			}
		} else {
			if result != nil {
				t.Errorf("Match(%q) expected nil, got %q (score: %f)", tc.transcript, result.CommandName, result.Score)
			}
		}
	}
}
