package diff

import (
	"testing"
)

func TestFormatDiffForTUI(t *testing.T) {
	diffResult := DiffResult{
		Lines: []DiffLine{
			{Type: LineContext, OldNumber: 1, NewNumber: 1, Content: "unchanged line"},
			{Type: LineRemoved, OldNumber: 2, NewNumber: 0, Content: "old content"},
			{Type: LineAdded, OldNumber: 0, NewNumber: 2, Content: "new content"},
			{Type: LineContext, OldNumber: 3, NewNumber: 3, Content: "another unchanged line"},
		},
		Added:   1,
		Removed: 1,
		OldName: "test.txt",
		NewName: "test.txt",
	}

	styledLines := FormatDiffForTUI(&diffResult, 80)

	// Should have header lines plus diff lines
	expectedMinLines := 2 + len(diffResult.Lines)
	if len(styledLines) < expectedMinLines {
		t.Errorf("Expected at least %d lines, got %d", expectedMinLines, len(styledLines))
	}

	// Check header lines
	if styledLines[0].Foreground != "bright_black" {
		t.Errorf("Expected first header line to have bright_black foreground, got %s", styledLines[0].Foreground)
	}
	if styledLines[0].Prefix != "" {
		t.Errorf("Header lines should have empty prefix, got %q", styledLines[0].Prefix)
	}

	if styledLines[1].Foreground != "bright_black" {
		t.Errorf("Expected second header line to have bright_black foreground, got %s", styledLines[1].Foreground)
	}

	// Find and check the removed and added lines
	var foundRemoved, foundAdded bool
	for _, line := range styledLines {
		if line.Background == "dark_red" {
			foundRemoved = true
			if line.Prefix != "-" {
				t.Errorf("Removed line prefix = %q, want %q", line.Prefix, "-")
			}
			if line.Foreground != "white" {
				t.Errorf("Removed line should have white foreground, got %s", line.Foreground)
			}
		}
		if line.Background == "dark_green" {
			foundAdded = true
			if line.Prefix != "+" {
				t.Errorf("Added line prefix = %q, want %q", line.Prefix, "+")
			}
			if line.Foreground != "white" {
				t.Errorf("Added line should have white foreground, got %s", line.Foreground)
			}
		}
	}

	if !foundRemoved {
		t.Error("Should have found a line with dark_red background (removed)")
	}
	if !foundAdded {
		t.Error("Should have found a line with dark_green background (added)")
	}
}

func TestFormatDiffForTUIContextLinePrefix(t *testing.T) {
	diffResult := DiffResult{
		Lines: []DiffLine{
			{Type: LineContext, OldNumber: 1, NewNumber: 1, Content: "unchanged"},
		},
	}

	styledLines := FormatDiffForTUI(&diffResult, 80)
	if len(styledLines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(styledLines))
	}
	if styledLines[0].Prefix != " " {
		t.Errorf("context line prefix = %q, want %q", styledLines[0].Prefix, " ")
	}
	if styledLines[0].Background != "" {
		t.Errorf("context line should have no background, got %q", styledLines[0].Background)
	}
}

func TestFormatDiffForTUIPrefixNotInText(t *testing.T) {
	diffResult := DiffResult{
		Lines: []DiffLine{
			{Type: LineAdded, OldNumber: 0, NewNumber: 1, Content: "new line"},
			{Type: LineRemoved, OldNumber: 1, NewNumber: 0, Content: "old line"},
			{Type: LineContext, OldNumber: 2, NewNumber: 2, Content: "ctx"},
		},
	}

	styledLines := FormatDiffForTUI(&diffResult, 80)
	for _, sl := range styledLines {
		// Text should NOT start with +/- prefix (it's in Prefix field)
		if len(sl.Text) > 0 && (sl.Text[0] == '+' || sl.Text[0] == '-') && sl.Prefix != "" {
			// Only flag if the first char of Text matches the prefix
			if string(sl.Text[0]) == sl.Prefix {
				t.Errorf("Text %q should not start with prefix %q", sl.Text, sl.Prefix)
			}
		}
	}
}

func TestFormatDiffForTUIWithNilDiff(t *testing.T) {
	styledLines := FormatDiffForTUI(nil, 80)
	if styledLines != nil {
		t.Error("Expected nil result for nil diff")
	}
}

func TestFormatDiffForTUIWithEmptyDiff(t *testing.T) {
	diffResult := &DiffResult{}
	styledLines := FormatDiffForTUI(diffResult, 80)
	if styledLines != nil {
		t.Error("Expected nil result for empty diff")
	}
}

func TestFormatDiffHeader(t *testing.T) {
	tests := []struct {
		name     string
		diff     *DiffResult
		expected string
	}{
		{
			name:     "nil diff",
			diff:     nil,
			expected: "",
		},
		{
			name:     "no changes",
			diff:     &DiffResult{Added: 0, Removed: 0},
			expected: "No changes",
		},
		{
			name: "with changes",
			diff: &DiffResult{
				Added:   2,
				Removed: 1,
				NewName: "test.txt",
			},
			expected: "test.txt (+2 -1)",
		},
		{
			name: "with path",
			diff: &DiffResult{
				Added:   1,
				Removed: 0,
				NewName: "/path/to/file.go",
			},
			expected: "file.go (+1 -0)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDiffHeader(tt.diff)
			if result != tt.expected {
				t.Errorf("FormatDiffHeader() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTruncateToWidth(t *testing.T) {
	tests := []struct {
		text     string
		width    int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10chars", 14, "exactly10chars"},
		{"this is too long for the width", 10, "this is..."},
		{"unicode: 你好世界", 8, "unico..."},
		{"short", 0, "short"}, // width 0 should return original
		{"", 5, ""},           // empty string
	}

	for _, tt := range tests {
		result := truncateToWidth(tt.text, tt.width)
		if result != tt.expected {
			t.Errorf("truncateToWidth(%q, %d) = %q, want %q",
				tt.text, tt.width, result, tt.expected)
		}
	}
}

func TestExpandTabs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		tabSize  int
		expected string
	}{
		{
			name:     "no tabs",
			input:    "hello world",
			tabSize:  4,
			expected: "hello world",
		},
		{
			name:     "single tab at start",
			input:    "\thello",
			tabSize:  4,
			expected: "    hello",
		},
		{
			name:     "tab after 1 char",
			input:    "x\thello",
			tabSize:  4,
			expected: "x   hello",
		},
		{
			name:     "tab after 3 chars",
			input:    "abc\thello",
			tabSize:  4,
			expected: "abc hello",
		},
		{
			name:     "tab after 4 chars",
			input:    "abcd\thello",
			tabSize:  4,
			expected: "abcd    hello",
		},
		{
			name:     "multiple tabs",
			input:    "\t\thello",
			tabSize:  4,
			expected: "        hello",
		},
		{
			name:     "tab size 8",
			input:    "\thello",
			tabSize:  8,
			expected: "        hello",
		},
		{
			name:     "mixed content",
			input:    "if x {\n\treturn\n}",
			tabSize:  4,
			expected: "if x {\n    return\n}",
		},
		{
			name:     "empty string",
			input:    "",
			tabSize:  4,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandTabs(tt.input, tt.tabSize)
			if result != tt.expected {
				t.Errorf("expandTabs(%q, %d) = %q, want %q",
					tt.input, tt.tabSize, result, tt.expected)
			}
		})
	}
}

func TestFormatDiffLineExpandsTabs(t *testing.T) {
	line := DiffLine{
		Type:    LineAdded,
		Content: "\tfmt.Println(\"hello\")",
	}

	styled := formatDiffLine(line, 80)

	if styled.Prefix != "+" {
		t.Errorf("prefix = %q, want %q", styled.Prefix, "+")
	}
	// Tab should be expanded to spaces
	expected := "    fmt.Println(\"hello\")"
	if styled.Text != expected {
		t.Errorf("Text = %q, want %q", styled.Text, expected)
	}
}
