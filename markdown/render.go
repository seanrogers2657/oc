package markdown

import (
	"strings"
	"unicode/utf8"
)

// SpanKind identifies the styling of a text span.
type SpanKind int

const (
	KindNormal     SpanKind = iota
	KindBold                // **text**
	KindItalic              // *text*
	KindBoldItalic          // ***text***
	KindCode                // `code`
	KindCodeBlock           // fenced code block content
	KindCodeLang            // language label on code fence
	KindHeader              // # heading
	KindLink                // [text]
	KindLinkURL             // (url)
	KindBlockquote          // > quoted text
	KindListBullet          // bullet/number marker
	KindHRule               // ---
)

// Span is a styled segment of text within a line.
type Span struct {
	Text string
	Kind SpanKind
}

// Line is one rendered line of output.
type Line struct {
	Spans  []Span
	Indent int // leading spaces
}

// Render parses markdown text and returns styled lines.
// This is a line-oriented parser; it does not produce a full AST.
func Render(input string) []Line {
	rawLines := strings.Split(input, "\n")
	var result []Line

	inCodeBlock := false
	codeLang := ""

	for i := 0; i < len(rawLines); i++ {
		line := rawLines[i]

		// Fenced code blocks
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if !inCodeBlock {
				inCodeBlock = true
				codeLang = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "```"))
				spans := []Span{{Text: "```", Kind: KindCodeBlock}}
				if codeLang != "" {
					spans = append(spans, Span{Text: codeLang, Kind: KindCodeLang})
				}
				result = append(result, Line{Spans: spans})
			} else {
				inCodeBlock = false
				codeLang = ""
				result = append(result, Line{Spans: []Span{{Text: "```", Kind: KindCodeBlock}}})
			}
			continue
		}

		if inCodeBlock {
			result = append(result, Line{Spans: []Span{{Text: line, Kind: KindCodeBlock}}})
			continue
		}

		// Horizontal rule
		trimmed := strings.TrimSpace(line)
		if isHRule(trimmed) {
			result = append(result, Line{Spans: []Span{{Text: trimmed, Kind: KindHRule}}})
			continue
		}

		// Headers
		if strings.HasPrefix(trimmed, "#") {
			level, text := parseHeader(trimmed)
			if level > 0 {
				prefix := strings.Repeat("#", level) + " "
				result = append(result, Line{Spans: []Span{
					{Text: prefix, Kind: KindHeader},
					{Text: text, Kind: KindHeader},
				}})
				continue
			}
		}

		// Blockquote
		if strings.HasPrefix(trimmed, "> ") || trimmed == ">" {
			text := strings.TrimPrefix(trimmed, "> ")
			if trimmed == ">" {
				text = ""
			}
			spans := []Span{{Text: "> ", Kind: KindBlockquote}}
			spans = append(spans, parseInline(text)...)
			result = append(result, Line{Spans: spans})
			continue
		}

		// Unordered list
		if idx, bullet := parseUnorderedList(trimmed); idx >= 0 {
			indent := countLeadingSpaces(line)
			spans := []Span{{Text: bullet + " ", Kind: KindListBullet}}
			text := trimmed[idx:]
			spans = append(spans, parseInline(text)...)
			result = append(result, Line{Spans: spans, Indent: indent})
			continue
		}

		// Ordered list
		if idx, marker := parseOrderedList(trimmed); idx >= 0 {
			indent := countLeadingSpaces(line)
			spans := []Span{{Text: marker + " ", Kind: KindListBullet}}
			text := trimmed[idx:]
			spans = append(spans, parseInline(text)...)
			result = append(result, Line{Spans: spans, Indent: indent})
			continue
		}

		// Regular paragraph line
		indent := countLeadingSpaces(line)
		if trimmed == "" {
			result = append(result, Line{})
		} else {
			result = append(result, Line{Spans: parseInline(trimmed), Indent: indent})
		}
	}

	return result
}

// parseInline handles inline formatting: bold, italic, code, links.
func parseInline(text string) []Span {
	var spans []Span
	var buf strings.Builder

	flush := func(kind SpanKind) {
		if buf.Len() > 0 {
			spans = append(spans, Span{Text: buf.String(), Kind: kind})
			buf.Reset()
		}
	}

	i := 0
	for i < len(text) {
		// Inline code
		if text[i] == '`' {
			flush(KindNormal)
			end := strings.Index(text[i+1:], "`")
			if end >= 0 {
				spans = append(spans, Span{Text: text[i+1 : i+1+end], Kind: KindCode})
				i = i + 1 + end + 1
				continue
			}
		}

		// Bold+italic ***text***
		if i+2 < len(text) && text[i:i+3] == "***" {
			flush(KindNormal)
			end := strings.Index(text[i+3:], "***")
			if end >= 0 {
				spans = append(spans, Span{Text: text[i+3 : i+3+end], Kind: KindBoldItalic})
				i = i + 3 + end + 3
				continue
			}
		}

		// Bold **text**
		if i+1 < len(text) && text[i:i+2] == "**" {
			flush(KindNormal)
			end := strings.Index(text[i+2:], "**")
			if end >= 0 {
				spans = append(spans, Span{Text: text[i+2 : i+2+end], Kind: KindBold})
				i = i + 2 + end + 2
				continue
			}
		}

		// Italic *text* (but not **)
		if text[i] == '*' && (i+1 >= len(text) || text[i+1] != '*') {
			flush(KindNormal)
			end := strings.Index(text[i+1:], "*")
			if end >= 0 && end > 0 {
				spans = append(spans, Span{Text: text[i+1 : i+1+end], Kind: KindItalic})
				i = i + 1 + end + 1
				continue
			}
		}

		// Link [text](url)
		if text[i] == '[' {
			flush(KindNormal)
			closeBracket := strings.Index(text[i:], "](")
			if closeBracket >= 0 {
				closeParen := strings.Index(text[i+closeBracket+2:], ")")
				if closeParen >= 0 {
					linkText := text[i+1 : i+closeBracket]
					linkURL := text[i+closeBracket+2 : i+closeBracket+2+closeParen]
					spans = append(spans, Span{Text: linkText, Kind: KindLink})
					spans = append(spans, Span{Text: " (" + linkURL + ")", Kind: KindLinkURL})
					i = i + closeBracket + 2 + closeParen + 1
					continue
				}
			}
		}

		// Regular character
		_, size := utf8.DecodeRuneInString(text[i:])
		buf.WriteString(text[i : i+size])
		i += size
	}

	flush(KindNormal)
	return spans
}

// parseHeader parses a # heading and returns (level, text).
func parseHeader(line string) (int, string) {
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level > 6 || level >= len(line) || line[level] != ' ' {
		return 0, ""
	}
	return level, strings.TrimSpace(line[level+1:])
}

// isHRule checks if a line is a horizontal rule (---, ***, ___).
func isHRule(line string) bool {
	if len(line) < 3 {
		return false
	}
	ch := line[0]
	if ch != '-' && ch != '*' && ch != '_' {
		return false
	}
	for _, r := range line {
		if r != rune(ch) && r != ' ' {
			return false
		}
	}
	count := strings.Count(line, string(ch))
	return count >= 3
}

// parseUnorderedList checks if line starts with "- ", "* ", or "+ ".
// Returns (index into line after marker, marker string) or (-1, "").
func parseUnorderedList(line string) (int, string) {
	stripped := strings.TrimLeft(line, " \t")
	if len(stripped) < 2 {
		return -1, ""
	}
	if (stripped[0] == '-' || stripped[0] == '*' || stripped[0] == '+') && stripped[1] == ' ' {
		offset := len(line) - len(stripped)
		return offset + 2, string(stripped[0])
	}
	return -1, ""
}

// parseOrderedList checks if line starts with "1. ", "2. ", etc.
// Returns (index into line after marker, marker string) or (-1, "").
func parseOrderedList(line string) (int, string) {
	stripped := strings.TrimLeft(line, " \t")
	dotIdx := strings.Index(stripped, ". ")
	if dotIdx < 1 || dotIdx > 9 {
		return -1, ""
	}
	num := stripped[:dotIdx]
	for _, r := range num {
		if r < '0' || r > '9' {
			return -1, ""
		}
	}
	offset := len(line) - len(stripped)
	marker := num + "."
	return offset + dotIdx + 2, marker
}

// countLeadingSpaces counts spaces at the start of a string.
func countLeadingSpaces(s string) int {
	n := 0
	for _, r := range s {
		if r == ' ' {
			n++
		} else if r == '\t' {
			n += 4
		} else {
			break
		}
	}
	return n
}
