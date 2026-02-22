package common

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestWrapTextASCII(t *testing.T) {
	lines := WrapText("hello world", 80)
	if len(lines) != 1 || lines[0] != "hello world" {
		t.Fatalf("expected single line 'hello world', got %v", lines)
	}
}

func TestWrapTextExactWidth(t *testing.T) {
	lines := WrapText("abcde", 5)
	if len(lines) != 1 || lines[0] != "abcde" {
		t.Fatalf("expected single line 'abcde', got %v", lines)
	}
}

func TestWrapTextOverflow(t *testing.T) {
	lines := WrapText("abcdefgh", 5)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "abcde" || lines[1] != "fgh" {
		t.Fatalf("expected [abcde, fgh], got %v", lines)
	}
}

func TestWrapTextMultiByte(t *testing.T) {
	text := "—————" // 5 em-dashes
	lines := WrapText(text, 5)
	if len(lines) != 1 {
		t.Fatalf("5 runes at width 5 should be 1 line, got %d", len(lines))
	}
}

func TestWrapTextMultiByteWrap(t *testing.T) {
	text := "————————" // 8 em-dashes
	lines := WrapText(text, 5)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if utf8.RuneCountInString(lines[0]) != 5 {
		t.Fatalf("first line should be 5 runes, got %d", utf8.RuneCountInString(lines[0]))
	}
}

func TestWrapTextEmpty(t *testing.T) {
	lines := WrapText("", 80)
	if lines != nil {
		t.Fatalf("empty text should return nil, got %v", lines)
	}
}

func TestWrapTextZeroWidth(t *testing.T) {
	// Zero width should default to 80
	lines := WrapText("hello", 0)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line with default width, got %d", len(lines))
	}
}

func TestWrapTextNegativeWidth(t *testing.T) {
	lines := WrapText("hello", -5)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line with default width, got %d", len(lines))
	}
}

func TestWrapTextNewlines(t *testing.T) {
	lines := WrapText("line1\nline2\nline3", 80)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "line1" || lines[1] != "line2" || lines[2] != "line3" {
		t.Fatalf("expected [line1, line2, line3], got %v", lines)
	}
}

func TestWrapTextNewlinesThenWrap(t *testing.T) {
	// Each line also wraps
	lines := WrapText("abcde\nfghij", 3)
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %v", len(lines), lines)
	}
}

func TestExpandTabsNoTabs(t *testing.T) {
	got := ExpandTabs("hello", 4)
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestExpandTabsBasic(t *testing.T) {
	tests := []struct {
		input    string
		tabWidth int
		want     string
	}{
		{"\t", 4, "    "},
		{"a\tb", 4, "a   b"},
		{"ab\tc", 4, "ab  c"},
		{"abc\td", 4, "abc d"},
		{"abcd\te", 4, "abcd    e"},
		{"\t\t", 4, "        "},
	}
	for _, tt := range tests {
		got := ExpandTabs(tt.input, tt.tabWidth)
		if got != tt.want {
			t.Errorf("ExpandTabs(%q, %d) = %q, want %q", tt.input, tt.tabWidth, got, tt.want)
		}
	}
}

func TestExpandTabsWidth2(t *testing.T) {
	got := ExpandTabs("\t", 2)
	if got != "  " {
		t.Errorf("expected 2 spaces, got %q", got)
	}
	got = ExpandTabs("a\t", 2)
	if got != "a " {
		t.Errorf("expected 'a ', got %q", got)
	}
}

func TestSplitLinesEmpty(t *testing.T) {
	lines := SplitLines("")
	if lines != nil {
		t.Fatalf("expected nil for empty string, got %v", lines)
	}
}

func TestSplitLinesSingleLine(t *testing.T) {
	lines := SplitLines("hello")
	if len(lines) != 1 || lines[0] != "hello" {
		t.Fatalf("expected [hello], got %v", lines)
	}
}

func TestSplitLinesMultiple(t *testing.T) {
	lines := SplitLines("a\nb\nc")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "a" || lines[1] != "b" || lines[2] != "c" {
		t.Fatalf("expected [a, b, c], got %v", lines)
	}
}

func TestSplitLinesTrailingNewline(t *testing.T) {
	lines := SplitLines("hello\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "hello" || lines[1] != "" {
		t.Fatalf("expected [hello, ''], got %v", lines)
	}
}

func TestSplitLinesLeadingNewline(t *testing.T) {
	lines := SplitLines("\nhello")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "" || lines[1] != "hello" {
		t.Fatalf("expected ['', hello], got %v", lines)
	}
}

func TestSplitLinesConsecutiveNewlines(t *testing.T) {
	lines := SplitLines("a\n\nb")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[1] != "" {
		t.Fatalf("middle line should be empty, got %q", lines[1])
	}
}

func TestWrapTextWithTabs(t *testing.T) {
	text := "\thello"
	lines := WrapText(text, 10)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if strings.Contains(lines[0], "\t") {
		t.Error("WrapText should expand tabs")
	}
	if lines[0] != "    hello" {
		t.Errorf("expected '    hello', got %q", lines[0])
	}
}
