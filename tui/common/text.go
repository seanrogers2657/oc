package common

import "strings"

// WrapText splits text into lines of at most maxWidth display columns.
// Tab characters are expanded to spaces (4-column tab stops) so they
// occupy the correct number of display columns.
func WrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		maxWidth = 80
	}

	var result []string
	for _, line := range SplitLines(text) {
		line = ExpandTabs(line, 4)
		runes := []rune(line)
		if len(runes) <= maxWidth {
			result = append(result, line)
		} else {
			for len(runes) > maxWidth {
				result = append(result, string(runes[:maxWidth]))
				runes = runes[maxWidth:]
			}
			if len(runes) > 0 {
				result = append(result, string(runes))
			}
		}
	}
	return result
}

// ExpandTabs replaces tab characters with spaces aligned to tabWidth-column stops.
func ExpandTabs(s string, tabWidth int) string {
	if !strings.Contains(s, "\t") {
		return s
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			spaces := tabWidth - (col % tabWidth)
			for i := 0; i < spaces; i++ {
				b.WriteByte(' ')
			}
			col += spaces
		} else {
			b.WriteRune(r)
			col++
		}
	}
	return b.String()
}

// SplitLines splits on \n, like strings.Split but returns nil for empty input.
func SplitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}
