package matcher

import (
	"math"
	"strings"
	"voicecmd/internal/config"
	"voicecmd/internal/stt"
)

// Result holds the outcome of a command match.
type Result struct {
	CommandName   string
	CommandConfig config.CommandConfig
	MatchedPhrase string
	Score         float64
}

// Match finds the best matching command for the given transcription.
// Returns nil if no command matches or if the highest score is below threshold.
func Match(transcript string, commands map[string]config.CommandConfig, threshold float64) *Result {
	cleanTranscript := stt.CleanTranscript(transcript)
	if cleanTranscript == "" || len(commands) == 0 {
		return nil
	}

	if threshold <= 0 {
		threshold = 0.75
	}

	var bestResult *Result

	for name, cmd := range commands {
		for _, phrase := range cmd.Phrases {
			cleanPhrase := stt.CleanTranscript(phrase)
			if cleanPhrase == "" {
				continue
			}

			score := calculateSimilarity(cleanTranscript, cleanPhrase)
			if bestResult == nil || score > bestResult.Score {
				bestResult = &Result{
					CommandName:   name,
					CommandConfig: cmd,
					MatchedPhrase: phrase,
					Score:         score,
				}
			}
		}
	}

	if bestResult != nil && bestResult.Score >= threshold {
		return bestResult
	}

	return nil
}

// calculateSimilarity computes a hybrid similarity score (0.0 to 1.0)
// considering exact match, substring inclusion, word token overlap, and Levenshtein distance.
func calculateSimilarity(transcript, phrase string) float64 {
	// 1. Exact match
	if transcript == phrase {
		return 1.0
	}

	// 2. Substring matching (e.g., "please open terminal" contains "open terminal")
	if strings.Contains(transcript, phrase) {
		// Proportion of phrase to transcript length (weighted heavily)
		ratio := float64(len(phrase)) / float64(len(transcript))
		return 0.85 + (0.15 * ratio) // Between 0.85 and 1.0
	}
	if strings.Contains(phrase, transcript) {
		ratio := float64(len(transcript)) / float64(len(phrase))
		return 0.80 + (0.15 * ratio)
	}

	// 3. Word token matching
	transWords := strings.Fields(transcript)
	phraseWords := strings.Fields(phrase)
	tokenScore := calculateTokenOverlap(transWords, phraseWords)

	// 4. Levenshtein edit distance similarity
	dist := levenshteinDistance(transcript, phrase)
	maxLen := math.Max(float64(len(transcript)), float64(len(phrase)))
	levScore := 0.0
	if maxLen > 0 {
		levScore = 1.0 - (float64(dist) / maxLen)
	}

	// Return the maximum of token overlap and character edit similarity
	return math.Max(tokenScore, levScore)
}

// calculateTokenOverlap checks how many words of the phrase exist in transcript in order.
func calculateTokenOverlap(transWords, phraseWords []string) float64 {
	if len(phraseWords) == 0 || len(transWords) == 0 {
		return 0
	}

	matchedWords := 0
	ti := 0
	for _, pw := range phraseWords {
		for ti < len(transWords) {
			if transWords[ti] == pw {
				matchedWords++
				ti++
				break
			}
			ti++
		}
	}

	if matchedWords == len(phraseWords) {
		// All phrase words appeared in order!
		// Penalize slightly if transcript has many extra words.
		ratio := float64(len(phraseWords)) / float64(len(transWords))
		return 0.80 + (0.15 * ratio)
	}

	// Jaccard-like word overlap
	phraseSet := make(map[string]bool)
	for _, w := range phraseWords {
		phraseSet[w] = true
	}
	common := 0
	for _, w := range transWords {
		if phraseSet[w] {
			common++
		}
	}

	return float64(common) / float64(len(phraseWords)+len(transWords)-common)
}

func levenshteinDistance(s1, s2 string) int {
	r1, r2 := []rune(s1), []rune(s2)
	n, m := len(r1), len(r2)

	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
		dp[i][0] = i
	}
	for j := 0; j <= m; j++ {
		dp[0][j] = j
	}

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			cost := 1
			if r1[i-1] == r2[j-1] {
				cost = 0
			}
			dp[i][j] = min(
				dp[i-1][j]+1,      // deletion
				dp[i][j-1]+1,      // insertion
				dp[i-1][j-1]+cost, // substitution
			)
		}
	}

	return dp[n][m]
}

func min(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
