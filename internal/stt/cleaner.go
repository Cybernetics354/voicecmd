package stt

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	// Matches bracketed or parenthesized whisper hallucination tags like (music), [BLANK_AUDIO], etc.
	noisePattern = regexp.MustCompile(`(?i)\([^\)]*\)|\[[^\]]*\]`)
	// Matches multiple whitespace characters
	spacePattern = regexp.MustCompile(`\s+`)
)

// CleanTranscript cleans raw transcription text by removing whisper artifacts,
// punctuation, and normalizing whitespace to lowercase.
func CleanTranscript(raw string) string {
	// Remove parenthesized or bracketed noise tags like (music), (dramatic music)
	cleaned := noisePattern.ReplaceAllString(raw, " ")

	// Replace non-alphanumeric characters (except spaces) with space or remove them
	var b strings.Builder
	for _, r := range cleaned {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(' ')
		}
	}

	// Collapse whitespace
	collapsed := spacePattern.ReplaceAllString(b.String(), " ")
	return strings.TrimSpace(collapsed)
}
