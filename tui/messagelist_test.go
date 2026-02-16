package tui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/srogers/oc/markdown"
)

func TestWrapTextASCII(t *testing.T) {
	lines := wrapText("hello world", 80)
	if len(lines) != 1 || lines[0] != "hello world" {
		t.Fatalf("expected single line 'hello world', got %v", lines)
	}
}

func TestWrapTextExactWidth(t *testing.T) {
	lines := wrapText("abcde", 5)
	if len(lines) != 1 || lines[0] != "abcde" {
		t.Fatalf("expected single line 'abcde', got %v", lines)
	}
}

func TestWrapTextOverflow(t *testing.T) {
	lines := wrapText("abcdefgh", 5)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "abcde" || lines[1] != "fgh" {
		t.Fatalf("expected [abcde, fgh], got %v", lines)
	}
}

func TestWrapTextMultiByte(t *testing.T) {
	// 5 em-dashes, each 3 bytes in UTF-8 but 1 display column
	text := "—————"
	if len(text) != 15 {
		t.Fatalf("expected 15 bytes, got %d", len(text))
	}
	if utf8.RuneCountInString(text) != 5 {
		t.Fatalf("expected 5 runes, got %d", utf8.RuneCountInString(text))
	}

	lines := wrapText(text, 5)
	if len(lines) != 1 {
		t.Fatalf("5 runes at width 5 should be 1 line, got %d: %v", len(lines), lines)
	}
	if lines[0] != text {
		t.Fatalf("expected %q, got %q", text, lines[0])
	}
}

func TestWrapTextMultiByteWrap(t *testing.T) {
	// 8 em-dashes at width 5 should wrap to 2 lines (5 + 3)
	text := "————————"
	lines := wrapText(text, 5)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if utf8.RuneCountInString(lines[0]) != 5 {
		t.Fatalf("first line should be 5 runes, got %d", utf8.RuneCountInString(lines[0]))
	}
	if utf8.RuneCountInString(lines[1]) != 3 {
		t.Fatalf("second line should be 3 runes, got %d", utf8.RuneCountInString(lines[1]))
	}
}

func TestWrapTextNoCorruptedRunes(t *testing.T) {
	// Wrapping multi-byte text must not produce invalid UTF-8
	text := "café résumé naïve"
	lines := wrapText(text, 6)
	for i, line := range lines {
		if !utf8.ValidString(line) {
			t.Errorf("line %d is invalid UTF-8: %q", i, line)
		}
	}
}

func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		input string
		n     int
		want  string
	}{
		{"hello", 3, "hel"},
		{"hello", 10, "hello"},
		{"hello", 0, ""},
		{"café", 3, "caf"},
		{"café", 4, "café"},
		{"———", 2, "——"},
		{"", 5, ""},
	}
	for _, tt := range tests {
		got := truncateRunes(tt.input, tt.n)
		if got != tt.want {
			t.Errorf("truncateRunes(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
		}
	}
}

func TestMdLineToRenderedMultiByte(t *testing.T) {
	// A markdown line with multi-byte content should not be split prematurely
	text := "Here's an em-dash: — and more text after"
	runeCount := utf8.RuneCountInString(text)

	line := markdown.Line{
		Spans: []markdown.Span{{Text: text, Kind: markdown.KindNormal}},
	}

	// Width larger than rune count — should be single line
	result := mdLineToRendered(line, runeCount+10)
	if len(result) != 1 {
		t.Fatalf("expected 1 line for text that fits, got %d", len(result))
	}
	// Verify the full text is in the rendered spans
	var rendered string
	for _, s := range result[0].spans {
		rendered += s.text
	}
	if rendered != text {
		t.Errorf("rendered text mismatch:\n  got:  %q\n  want: %q", rendered, text)
	}
}

func TestMdLineToRenderedWrapsCorrectly(t *testing.T) {
	// "aaaa————bbbb" = 12 runes, at width 8 should wrap
	text := "aaaa————bbbb"
	if utf8.RuneCountInString(text) != 12 {
		t.Fatalf("expected 12 runes, got %d", utf8.RuneCountInString(text))
	}

	line := markdown.Line{
		Spans: []markdown.Span{{Text: text, Kind: markdown.KindNormal}},
	}
	result := mdLineToRendered(line, 8)
	if len(result) != 2 {
		t.Fatalf("expected 2 lines at width 8, got %d", len(result))
	}

	// Verify no rune corruption in any line
	for i, rl := range result {
		for _, s := range rl.spans {
			if !utf8.ValidString(s.text) {
				t.Errorf("line %d has invalid UTF-8: %q", i, s.text)
			}
		}
	}

	// Verify total content is preserved
	var total string
	for _, rl := range result {
		for _, s := range rl.spans {
			total += s.text
		}
	}
	if !strings.Contains(total, "aaaa") || !strings.Contains(total, "bbbb") {
		t.Errorf("content lost after wrapping: %q", total)
	}
}

func TestExpandTabs(t *testing.T) {
	tests := []struct {
		input    string
		tabWidth int
		want     string
	}{
		{"no tabs", 4, "no tabs"},
		{"\t", 4, "    "},
		{"a\tb", 4, "a   b"},
		{"ab\tc", 4, "ab  c"},
		{"abc\td", 4, "abc d"},
		{"abcd\te", 4, "abcd    e"},
		{"\t\t", 4, "        "},
		{"    1\tline", 4, "    1   line"}, // mimics read.go format
	}
	for _, tt := range tests {
		got := expandTabs(tt.input, tt.tabWidth)
		if got != tt.want {
			t.Errorf("expandTabs(%q, %d) = %q, want %q", tt.input, tt.tabWidth, got, tt.want)
		}
	}
}

func TestWrapTextWithTabs(t *testing.T) {
	// Tab at column 0 expands to 4 spaces; total = 4 + 5 = 9 runes
	text := "\thello"
	lines := wrapText(text, 10)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %v", len(lines), lines)
	}
	if strings.Contains(lines[0], "\t") {
		t.Error("wrapText should expand tabs; output still contains \\t")
	}
	if lines[0] != "    hello" {
		t.Errorf("expected '    hello', got %q", lines[0])
	}
}

func TestWrapTextTabNotSplitMidExpansion(t *testing.T) {
	// "12345678\tX" → tab at col 8 expands to 4 spaces → "12345678    X" (13 runes)
	// At width 10, wraps to: "1234567812" + "  X" — no, wait.
	// expandTabs runs first: "12345678" + 4 spaces + "X" = "12345678    X"
	// Then []rune has 13 runes. Wrapping at 10: "12345678  " + "  X"
	text := "12345678\tX"
	lines := wrapText(text, 10)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	// No raw tab characters should survive
	for i, l := range lines {
		if strings.Contains(l, "\t") {
			t.Errorf("line %d still contains tab: %q", i, l)
		}
	}
}
