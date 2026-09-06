package stt

import (
	"testing"
)

func TestCleanTranscript(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{" Shut down.", "shut down"},
		{"(dramatic music)", ""},
		{"[BLANK_AUDIO]", ""},
		{"Hello, world! How are you?", "hello world how are you"},
		{"  Turn   up   volume!  ", "turn up volume"},
		{"(music) Open Firefox. (applause)", "open firefox"},
		{"mute", "mute"},
	}

	for _, tc := range tests {
		got := CleanTranscript(tc.input)
		if got != tc.expected {
			t.Errorf("CleanTranscript(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}
