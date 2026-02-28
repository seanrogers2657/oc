package markdown

import (
	"strings"
	"testing"
)

func spanText(spans []Span) string {
	var b strings.Builder
	for _, s := range spans {
		b.WriteString(s.Text)
	}
	return b.String()
}

func findSpanKind(spans []Span, kind SpanKind) *Span {
	for i := range spans {
		if spans[i].Kind == kind {
			return &spans[i]
		}
	}
	return nil
}

func TestRenderPlainText(t *testing.T) {
	lines := Render("Hello world")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if spanText(lines[0].Spans) != "Hello world" {
		t.Errorf("unexpected text: %q", spanText(lines[0].Spans))
	}
}

func TestRenderBold(t *testing.T) {
	lines := Render("This is **bold** text")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	bold := findSpanKind(lines[0].Spans, KindBold)
	if bold == nil {
		t.Fatal("expected bold span")
	}
	if bold.Text != "bold" {
		t.Errorf("expected 'bold', got %q", bold.Text)
	}
}

func TestRenderItalic(t *testing.T) {
	lines := Render("This is *italic* text")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	italic := findSpanKind(lines[0].Spans, KindItalic)
	if italic == nil {
		t.Fatal("expected italic span")
	}
	if italic.Text != "italic" {
		t.Errorf("expected 'italic', got %q", italic.Text)
	}
}

func TestRenderBoldItalic(t *testing.T) {
	lines := Render("This is ***both*** text")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	bi := findSpanKind(lines[0].Spans, KindBoldItalic)
	if bi == nil {
		t.Fatal("expected bold+italic span")
	}
	if bi.Text != "both" {
		t.Errorf("expected 'both', got %q", bi.Text)
	}
}

func TestRenderInlineCode(t *testing.T) {
	lines := Render("Use `fmt.Println` here")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	code := findSpanKind(lines[0].Spans, KindCode)
	if code == nil {
		t.Fatal("expected code span")
	}
	if code.Text != "fmt.Println" {
		t.Errorf("expected 'fmt.Println', got %q", code.Text)
	}
}

func TestRenderCodeBlock(t *testing.T) {
	input := "```go\nfunc main() {}\n```"
	lines := Render(input)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	// First line: ``` + lang
	langSpan := findSpanKind(lines[0].Spans, KindCodeLang)
	if langSpan == nil {
		t.Fatal("expected code lang span")
	}
	if langSpan.Text != "go" {
		t.Errorf("expected 'go', got %q", langSpan.Text)
	}
	// Middle line: code content
	if lines[1].Spans[0].Kind != KindCodeBlock {
		t.Error("expected code block span for content")
	}
	if lines[1].Spans[0].Text != "func main() {}" {
		t.Errorf("unexpected code content: %q", lines[1].Spans[0].Text)
	}
	// Last line: closing fence
	if lines[2].Spans[0].Kind != KindCodeBlock {
		t.Error("expected code block span for closing fence")
	}
}

func TestRenderHeader(t *testing.T) {
	for _, test := range []struct {
		input string
		level int
		text  string
	}{
		{"# Title", 1, "Title"},
		{"## Subtitle", 2, "Subtitle"},
		{"### H3", 3, "H3"},
	} {
		lines := Render(test.input)
		if len(lines) != 1 {
			t.Fatalf("input %q: expected 1 line, got %d", test.input, len(lines))
		}
		header := findSpanKind(lines[0].Spans, KindHeader)
		if header == nil {
			t.Fatalf("input %q: expected header span", test.input)
		}
		if header.Text != test.text {
			t.Errorf("input %q: expected header text %q, got %q", test.input, test.text, header.Text)
		}
		// Ensure # prefix is not in the output
		fullText := spanText(lines[0].Spans)
		if strings.Contains(fullText, "#") {
			t.Errorf("input %q: header should not contain # prefix, got %q", test.input, fullText)
		}
	}
}

func TestRenderHeaderWithInlineFormatting(t *testing.T) {
	lines := Render("### Why **Context** and *Approach*")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// Should have header spans for normal text, bold for **Context**, italic for *Approach*
	bold := findSpanKind(lines[0].Spans, KindBold)
	if bold == nil {
		t.Fatal("expected bold span within header")
	}
	if bold.Text != "Context" {
		t.Errorf("expected bold text 'Context', got %q", bold.Text)
	}
	italic := findSpanKind(lines[0].Spans, KindItalic)
	if italic == nil {
		t.Fatal("expected italic span within header")
	}
	if italic.Text != "Approach" {
		t.Errorf("expected italic text 'Approach', got %q", italic.Text)
	}
	// Normal text in header should be KindHeader
	header := findSpanKind(lines[0].Spans, KindHeader)
	if header == nil {
		t.Fatal("expected header span for normal text")
	}
	// No raw ** markers
	fullText := spanText(lines[0].Spans)
	if strings.Contains(fullText, "**") || strings.Contains(fullText, "##") {
		t.Errorf("header should not contain raw markers, got %q", fullText)
	}
}

func TestRenderBlockquote(t *testing.T) {
	lines := Render("> quoted text")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	bq := findSpanKind(lines[0].Spans, KindBlockquote)
	if bq == nil {
		t.Fatal("expected blockquote span")
	}
}

func TestRenderUnorderedList(t *testing.T) {
	lines := Render("- item one\n- item two")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	bullet := findSpanKind(lines[0].Spans, KindListBullet)
	if bullet == nil {
		t.Fatal("expected list bullet span")
	}
}

func TestRenderOrderedList(t *testing.T) {
	lines := Render("1. first\n2. second")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	bullet := findSpanKind(lines[0].Spans, KindListBullet)
	if bullet == nil {
		t.Fatal("expected list bullet span")
	}
	if !strings.Contains(bullet.Text, "1.") {
		t.Errorf("expected '1.' in bullet, got %q", bullet.Text)
	}
}

func TestRenderHRule(t *testing.T) {
	lines := Render("---")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	hr := findSpanKind(lines[0].Spans, KindHRule)
	if hr == nil {
		t.Fatal("expected hrule span")
	}
}

func TestRenderLink(t *testing.T) {
	lines := Render("[click here](https://example.com)")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	link := findSpanKind(lines[0].Spans, KindLink)
	if link == nil {
		t.Fatal("expected link span")
	}
	if link.Text != "click here" {
		t.Errorf("expected 'click here', got %q", link.Text)
	}
	url := findSpanKind(lines[0].Spans, KindLinkURL)
	if url == nil {
		t.Fatal("expected link URL span")
	}
	if !strings.Contains(url.Text, "example.com") {
		t.Errorf("expected URL to contain 'example.com', got %q", url.Text)
	}
}

func TestRenderEmptyLines(t *testing.T) {
	lines := Render("para one\n\npara two")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	// Middle line should be empty
	if len(lines[1].Spans) != 0 {
		t.Error("expected empty line between paragraphs")
	}
}

func TestRenderMixed(t *testing.T) {
	input := "# Hello\n\nThis is **bold** and *italic*.\n\n```\ncode\n```\n\n- list item"
	lines := Render(input)

	// Should produce multiple lines with headers, paragraphs, code blocks, lists
	if len(lines) < 5 {
		t.Fatalf("expected at least 5 lines, got %d", len(lines))
	}

	// First line should be header
	if findSpanKind(lines[0].Spans, KindHeader) == nil {
		t.Error("first line should be header")
	}
}

func TestRenderHRuleVariants(t *testing.T) {
	for _, input := range []string{"---", "***", "___", "- - -", "* * *"} {
		lines := Render(input)
		if len(lines) != 1 {
			t.Fatalf("input %q: expected 1 line, got %d", input, len(lines))
		}
		if findSpanKind(lines[0].Spans, KindHRule) == nil {
			t.Errorf("input %q: expected hrule", input)
		}
	}
}

func TestRenderEmpty(t *testing.T) {
	lines := Render("")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line for empty input, got %d", len(lines))
	}
}
