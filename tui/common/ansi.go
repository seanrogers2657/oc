package common

import (
	"fmt"
	"strings"
)

// Alt screen
const (
	AltScreenEnter = "\x1b[?1049h"
	AltScreenExit  = "\x1b[?1049l"
)

// Cursor visibility
const (
	CursorHide = "\x1b[?25l"
	CursorShow = "\x1b[?25h"
)

// Screen clearing
const (
	ClearScreen = "\x1b[2J"
	ClearLine   = "\x1b[2K"
)

// Bracketed paste mode
const (
	BracketedPasteEnable  = "\x1b[?2004h"
	BracketedPasteDisable = "\x1b[?2004l"
)


// Reset all attributes
const ResetStyle = "\x1b[0m"

// CursorTo moves the cursor to the given 0-based position.
func CursorTo(x, y int) string {
	return fmt.Sprintf("\x1b[%d;%dH", y+1, x+1)
}

// FGRGB returns an ANSI true-color foreground sequence.
func FGRGB(r, g, b uint8) string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
}

// BGRGB returns an ANSI true-color background sequence.
func BGRGB(r, g, b uint8) string {
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
}

// FG256 returns an ANSI 256-color foreground sequence.
func FG256(code uint8) string {
	return fmt.Sprintf("\x1b[38;5;%dm", code)
}

// BG256 returns an ANSI 256-color background sequence.
func BG256(code uint8) string {
	return fmt.Sprintf("\x1b[48;5;%dm", code)
}

// Attribute sequences
const (
	Bold          = "\x1b[1m"
	Dim           = "\x1b[2m"
	Italic        = "\x1b[3m"
	Underline     = "\x1b[4m"
	Strikethrough = "\x1b[9m"
)

// StyleToANSI converts a Style into the ANSI escape sequence that activates it.
func StyleToANSI(s Style) string {
	var b strings.Builder
	b.Grow(64)

	if s.Bold {
		b.WriteString(Bold)
	}
	if s.Dim {
		b.WriteString(Dim)
	}
	if s.Italic {
		b.WriteString(Italic)
	}
	if s.Underline {
		b.WriteString(Underline)
	}
	if s.Strikethrough {
		b.WriteString(Strikethrough)
	}
	if s.FG.Set {
		b.WriteString(FGRGB(s.FG.R, s.FG.G, s.FG.B))
	}
	if s.BG.Set {
		b.WriteString(BGRGB(s.BG.R, s.BG.G, s.BG.B))
	}

	return b.String()
}

// StylesEqual returns true if two styles produce the same visual output.
func StylesEqual(a, b Style) bool {
	return a == b
}
