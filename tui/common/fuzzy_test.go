package common

import "testing"

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		candidate string
		wantMatch bool
	}{
		{"empty query matches everything", "", "anything", true},
		{"exact match", "clear", "clear", true},
		{"prefix match", "cl", "clear", true},
		{"scattered chars", "clr", "clear", true},
		{"no match", "xyz", "clear", false},
		{"case insensitive", "CL", "clear", true},
		{"candidate shorter than query", "clearing", "clear", false},
		{"empty candidate", "a", "", false},
		{"both empty", "", "", true},
		{"unicode", "café", "café latte", true},
		{"alias match", "q", "quit", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, matched := FuzzyMatch(tt.query, tt.candidate)
			if matched != tt.wantMatch {
				t.Errorf("FuzzyMatch(%q, %q) matched=%v, want %v", tt.query, tt.candidate, matched, tt.wantMatch)
			}
			if matched && score <= 0 {
				t.Errorf("FuzzyMatch(%q, %q) matched but score=%d, want >0", tt.query, tt.candidate, score)
			}
		})
	}
}

func TestFuzzyMatchScoreOrdering(t *testing.T) {
	// "cl" should score "clear" higher than "cancel" (prefix match vs scattered)
	scoreClear, _ := FuzzyMatch("cl", "clear")
	scoreCancel, _ := FuzzyMatch("cl", "cancel")
	if scoreClear <= scoreCancel {
		t.Errorf("expected 'clear' (%d) to score higher than 'cancel' (%d) for query 'cl'",
			scoreClear, scoreCancel)
	}

	// Exact match should score highest
	scoreExact, _ := FuzzyMatch("exit", "exit")
	scorePartial, _ := FuzzyMatch("exit", "exit application")
	if scoreExact <= scorePartial {
		t.Errorf("expected exact 'exit' (%d) to score >= partial 'exit application' (%d)",
			scoreExact, scorePartial)
	}
}
