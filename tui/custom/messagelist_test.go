package custom

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/seanrogers2657/oc/markdown"
	"github.com/seanrogers2657/oc/tui/common"
)

func TestWrapTextASCII(t *testing.T) {
	lines := common.WrapText("hello world", 80)
	if len(lines) != 1 || lines[0] != "hello world" {
		t.Fatalf("expected single line 'hello world', got %v", lines)
	}
}

func TestWrapTextExactWidth(t *testing.T) {
	lines := common.WrapText("abcde", 5)
	if len(lines) != 1 || lines[0] != "abcde" {
		t.Fatalf("expected single line 'abcde', got %v", lines)
	}
}

func TestWrapTextOverflow(t *testing.T) {
	lines := common.WrapText("abcdefgh", 5)
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

	lines := common.WrapText(text, 5)
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
	lines := common.WrapText(text, 5)
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
	lines := common.WrapText(text, 6)
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

func TestFindBreakPoint(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxRunes int
		want     string
	}{
		{"text fits", "hello", 10, "hello"},
		{"space break", "hello world", 8, "hello"},
		{"hyphen break", "well-done", 8, "well-"},
		{"no break point", "abcdefghij", 5, ""},
		{"break at exact limit", "hello world", 5, "hello"},
		{"prefer last space", "one two three", 10, "one two"},
		{"hyphen mid-word", "self-contained stuff", 15, "self-contained"},
		{"single char", "a", 1, "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findBreakPoint(tt.text, tt.maxRunes)
			if got != tt.want {
				t.Errorf("findBreakPoint(%q, %d) = %q, want %q", tt.text, tt.maxRunes, got, tt.want)
			}
		})
	}
}

func TestMdLineToRenderedWordWrap(t *testing.T) {
	// "hello world foo" at width 12 should wrap to ["hello world", "foo"]
	line := markdown.Line{
		Spans: []markdown.Span{{Text: "hello world foo", Kind: markdown.KindNormal}},
	}
	result := mdLineToRendered(line, 12)
	if len(result) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(result))
	}
	row0 := spanText(result[0])
	row1 := spanText(result[1])
	if row0 != "hello world" {
		t.Errorf("row 0 = %q, want %q", row0, "hello world")
	}
	if row1 != "foo" {
		t.Errorf("row 1 = %q, want %q", row1, "foo")
	}
}

func TestMdLineToRenderedIndentPreservation(t *testing.T) {
	// Line with indent 4, text wraps — continuation should have 4-space prefix
	line := markdown.Line{
		Indent: 4,
		Spans:  []markdown.Span{{Text: "hello world foo bar", Kind: markdown.KindNormal}},
	}
	// Width 16: indent(4) + 12 content chars per line
	result := mdLineToRendered(line, 16)
	if len(result) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(result))
	}
	// Check continuation line starts with 4 spaces
	row1 := spanText(result[1])
	if !strings.HasPrefix(row1, "    ") {
		t.Errorf("continuation line should start with 4 spaces, got %q", row1)
	}
}

func TestMdLineToRenderedStyledSpanWrap(t *testing.T) {
	// Bold span crossing line boundary keeps bold style on both lines
	line := markdown.Line{
		Spans: []markdown.Span{{Text: "hello world foo", Kind: markdown.KindBold}},
	}
	result := mdLineToRendered(line, 12)
	if len(result) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(result))
	}
	boldStyle := mdStyle(markdown.KindBold)
	// Check both lines have the bold style
	for i, rl := range result {
		for _, s := range rl.spans {
			if s.text != "" && s.style != boldStyle {
				t.Errorf("line %d span %q: expected bold style, got %+v", i, s.text, s.style)
			}
		}
	}
}

func TestMdLineToRenderedLongWordForcedBreak(t *testing.T) {
	// Word longer than maxWidth falls back to character break
	line := markdown.Line{
		Spans: []markdown.Span{{Text: "abcdefghijklmnop", Kind: markdown.KindNormal}},
	}
	result := mdLineToRendered(line, 8)
	if len(result) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(result))
	}
	row0 := spanText(result[0])
	row1 := spanText(result[1])
	if row0 != "abcdefgh" {
		t.Errorf("row 0 = %q, want %q", row0, "abcdefgh")
	}
	if row1 != "ijklmnop" {
		t.Errorf("row 1 = %q, want %q", row1, "ijklmnop")
	}
}

func TestMdLineToRenderedHyphenBreak(t *testing.T) {
	// "well-documented code" at width 16 should break after hyphen
	line := markdown.Line{
		Spans: []markdown.Span{{Text: "well-documented code", Kind: markdown.KindNormal}},
	}
	result := mdLineToRendered(line, 16)
	if len(result) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(result))
	}
	row0 := spanText(result[0])
	row1 := spanText(result[1])
	if row0 != "well-documented" {
		t.Errorf("row 0 = %q, want %q", row0, "well-documented")
	}
	if row1 != "code" {
		t.Errorf("row 1 = %q, want %q", row1, "code")
	}
}

// spanText concatenates all span text in a renderedLine.
func spanText(rl renderedLine) string {
	var sb strings.Builder
	for _, s := range rl.spans {
		sb.WriteString(s.text)
	}
	return sb.String()
}

func TestScrollInfoDefaults(t *testing.T) {
	ml := NewMessageList(nil)
	offset, autoScroll := ml.ScrollInfo()
	if offset != 0 {
		t.Errorf("expected offset 0, got %d", offset)
	}
	if !autoScroll {
		t.Error("expected autoScroll true by default")
	}
}

func TestScrollInfoAfterManualScroll(t *testing.T) {
	ml := NewMessageList(nil)
	// Simulate content taller than viewport
	ml.scroll.ContentHeight = 100
	ml.scroll.ViewportHeight = 20
	ml.scroll.Offset = 0

	// Scroll up disables auto-scroll
	ml.ScrollUp(5)
	offset, autoScroll := ml.ScrollInfo()
	if autoScroll {
		t.Error("expected autoScroll false after ScrollUp")
	}
	if offset != 0 {
		// ScrollUp from 0 should clamp to 0
		t.Errorf("expected offset 0, got %d", offset)
	}

	// Set offset to simulate content that has scrolled
	ml.scroll.Offset = 50
	ml.autoScroll = false

	// ScrollDown to bottom re-enables auto-scroll
	ml.scroll.Offset = ml.scroll.ContentHeight - ml.scroll.ViewportHeight
	ml.ScrollDown(1)
	_, autoScroll = ml.ScrollInfo()
	if !autoScroll {
		t.Error("expected autoScroll true after scrolling to bottom")
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
		got := common.ExpandTabs(tt.input, tt.tabWidth)
		if got != tt.want {
			t.Errorf("ExpandTabs(%q, %d) = %q, want %q", tt.input, tt.tabWidth, got, tt.want)
		}
	}
}

func TestWrapTextWithTabs(t *testing.T) {
	// Tab at column 0 expands to 4 spaces; total = 4 + 5 = 9 runes
	text := "\thello"
	lines := common.WrapText(text, 10)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %v", len(lines), lines)
	}
	if strings.Contains(lines[0], "\t") {
		t.Error("WrapText should expand tabs; output still contains \\t")
	}
	if lines[0] != "    hello" {
		t.Errorf("expected '    hello', got %q", lines[0])
	}
}

func TestWrapTextTabNotSplitMidExpansion(t *testing.T) {
	text := "12345678\tX"
	lines := common.WrapText(text, 10)
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
