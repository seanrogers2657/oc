package tui

import "unicode"

// FuzzyMatch scores how well query matches candidate.
// Returns (score, matched). Empty query matches everything with score 1.
// Matching is case-insensitive, character-by-character left-to-right scan.
func FuzzyMatch(query, candidate string) (int, bool) {
	if len(query) == 0 {
		return 1, true
	}

	qRunes := []rune(query)
	cRunes := []rune(candidate)

	qi := 0 // index into query
	score := 0
	lastMatchIdx := -1

	for ci := 0; ci < len(cRunes) && qi < len(qRunes); ci++ {
		qr := unicode.ToLower(qRunes[qi])
		cr := unicode.ToLower(cRunes[ci])

		if qr != cr {
			continue
		}

		// Base match
		score++

		// Prefix bonus: query chars match at the start of candidate
		if ci == qi {
			score += 10
		}

		// Word boundary bonus: char after space, underscore, hyphen, or at start
		if ci == 0 || cRunes[ci-1] == ' ' || cRunes[ci-1] == '_' || cRunes[ci-1] == '-' {
			score += 5
		}

		// Consecutive char bonus
		if lastMatchIdx >= 0 && ci == lastMatchIdx+1 {
			score += 3
		}

		lastMatchIdx = ci
		qi++
	}

	if qi < len(qRunes) {
		return 0, false
	}

	// Exact match bonus: query is the entire candidate
	if len(qRunes) == len(cRunes) {
		score += 10
	}

	return score, true
}
