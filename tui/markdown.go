package tui

import "github.com/srogers/oc/markdown"

// mdStyle maps a markdown SpanKind to a TUI Style.
func mdStyle(kind markdown.SpanKind) Style {
	switch kind {
	case markdown.KindBold:
		return Style{FG: NewColor(220, 220, 220), Bold: true}
	case markdown.KindItalic:
		return Style{FG: NewColor(200, 200, 200), Italic: true}
	case markdown.KindBoldItalic:
		return Style{FG: NewColor(220, 220, 220), Bold: true, Italic: true}
	case markdown.KindCode:
		return Style{FG: NewColor(230, 180, 80), BG: NewColor(40, 40, 40)}
	case markdown.KindCodeBlock:
		return Style{FG: NewColor(200, 200, 200), BG: NewColor(30, 30, 30)}
	case markdown.KindCodeLang:
		return Style{FG: NewColor(120, 120, 120), BG: NewColor(30, 30, 30), Italic: true}
	case markdown.KindHeader:
		return Style{FG: NewColor(140, 170, 255), Bold: true}
	case markdown.KindLink:
		return Style{FG: NewColor(100, 180, 255), Underline: true}
	case markdown.KindLinkURL:
		return Style{FG: NewColor(100, 100, 100)}
	case markdown.KindBlockquote:
		return Style{FG: NewColor(160, 160, 160), Italic: true}
	case markdown.KindListBullet:
		return Style{FG: NewColor(100, 200, 100)}
	case markdown.KindHRule:
		return Style{FG: NewColor(80, 80, 80)}
	default:
		return Style{FG: NewColor(220, 220, 220)}
	}
}

// renderMarkdownLine writes one markdown Line to the screen buffer,
// returning the number of screen rows consumed (accounting for wrapping).
func renderMarkdownLine(buf *ScreenBuffer, x, y, maxWidth int, line markdown.Line) int {
	if maxWidth <= 0 {
		return 0
	}

	cx := x + line.Indent
	row := 0

	for _, span := range line.Spans {
		style := mdStyle(span.Kind)

		// For horizontal rules, draw a full-width line
		if span.Kind == markdown.KindHRule {
			for i := 0; i < maxWidth; i++ {
				buf.Set(x+i, y+row, '─', style)
			}
			return 1
		}

		for _, r := range span.Text {
			if cx >= x+maxWidth {
				// Wrap to next line
				row++
				cx = x + line.Indent
			}
			if r == '\t' {
				// Render tab as spaces
				for j := 0; j < 4 && cx < x+maxWidth; j++ {
					buf.Set(cx, y+row, ' ', style)
					cx++
				}
			} else {
				buf.Set(cx, y+row, r, style)
				cx++
			}
		}
	}

	return row + 1
}
