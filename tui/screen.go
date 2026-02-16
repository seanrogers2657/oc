package tui

import (
	"strings"
	"unicode/utf8"
)

// Color represents an RGB color for terminal rendering.
type Color struct {
	R, G, B uint8
	Set      bool // false = default/no color
}

// NewColor creates a Color with Set=true.
func NewColor(r, g, b uint8) Color {
	return Color{R: r, G: g, B: b, Set: true}
}

// Style describes how a cell or span of text should look.
type Style struct {
	FG            Color
	BG            Color
	Bold          bool
	Dim           bool
	Italic        bool
	Underline     bool
	Strikethrough bool
}

// Cell is a single character position in the screen buffer.
type Cell struct {
	Rune  rune
	Style Style
	Width int // 1=normal, 2=wide (CJK), 0=continuation of wide char
}

// ScreenBuffer is a 2D grid of cells representing the desired screen state.
type ScreenBuffer struct {
	Width  int
	Height int
	Cells  [][]Cell
}

// NewScreenBuffer creates a buffer filled with spaces.
func NewScreenBuffer(width, height int) *ScreenBuffer {
	buf := &ScreenBuffer{
		Width:  width,
		Height: height,
		Cells:  make([][]Cell, height),
	}
	for y := range buf.Cells {
		buf.Cells[y] = make([]Cell, width)
		for x := range buf.Cells[y] {
			buf.Cells[y][x] = Cell{Rune: ' ', Width: 1}
		}
	}
	return buf
}

// Clear resets all cells to spaces with default style.
func (b *ScreenBuffer) Clear() {
	for y := range b.Cells {
		for x := range b.Cells[y] {
			b.Cells[y][x] = Cell{Rune: ' ', Width: 1}
		}
	}
}

// Resize adjusts the buffer dimensions, preserving content where possible.
func (b *ScreenBuffer) Resize(width, height int) {
	newCells := make([][]Cell, height)
	for y := range newCells {
		newCells[y] = make([]Cell, width)
		for x := range newCells[y] {
			if y < b.Height && x < b.Width {
				newCells[y][x] = b.Cells[y][x]
			} else {
				newCells[y][x] = Cell{Rune: ' ', Width: 1}
			}
		}
	}
	b.Width = width
	b.Height = height
	b.Cells = newCells
}

// Set places a rune with style at (x, y). Out-of-bounds writes are ignored.
func (b *ScreenBuffer) Set(x, y int, r rune, s Style) {
	if y < 0 || y >= b.Height || x < 0 || x >= b.Width {
		return
	}
	b.Cells[y][x] = Cell{Rune: r, Style: s, Width: 1}
}

// WriteString writes a string horizontally starting at (x, y).
// Returns the number of columns consumed.
func (b *ScreenBuffer) WriteString(x, y int, text string, s Style) int {
	if y < 0 || y >= b.Height {
		return 0
	}
	col := x
	for _, r := range text {
		if col >= b.Width {
			break
		}
		if col >= 0 {
			b.Cells[y][col] = Cell{Rune: r, Style: s, Width: 1}
		}
		col++
	}
	return col - x
}

// Fill fills a rectangular region with a rune and style.
func (b *ScreenBuffer) Fill(bounds Rect, r rune, s Style) {
	for y := bounds.Y; y < bounds.Y+bounds.Height && y < b.Height; y++ {
		if y < 0 {
			continue
		}
		for x := bounds.X; x < bounds.X+bounds.Width && x < b.Width; x++ {
			if x < 0 {
				continue
			}
			b.Cells[y][x] = Cell{Rune: r, Style: s, Width: 1}
		}
	}
}

// Rect defines a rectangular region on screen.
type Rect struct {
	X, Y, Width, Height int
}

// WriteWrapped writes text with word wrapping within bounds.
// Returns the number of lines used.
func (b *ScreenBuffer) WriteWrapped(bounds Rect, text string, s Style) int {
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return 0
	}

	line := 0
	col := 0

	for _, r := range text {
		if r == '\n' {
			line++
			col = 0
			if line >= bounds.Height {
				break
			}
			continue
		}

		if col >= bounds.Width {
			line++
			col = 0
			if line >= bounds.Height {
				break
			}
		}

		py := bounds.Y + line
		px := bounds.X + col
		if py >= 0 && py < b.Height && px >= 0 && px < b.Width {
			b.Cells[py][px] = Cell{Rune: r, Style: s, Width: 1}
		}
		col++
	}

	return line + 1
}

// Diff produces ANSI escape codes that transform prev into b.
// Only changed cells emit output. This is the core of flicker-free rendering.
func (b *ScreenBuffer) Diff(prev *ScreenBuffer) string {
	var out strings.Builder
	out.Grow(b.Width * b.Height) // rough estimate

	var lastStyle Style
	styleActive := false
	lastX, lastY := -1, -1

	for y := 0; y < b.Height; y++ {
		for x := 0; x < b.Width; x++ {
			cell := b.Cells[y][x]

			// Skip if unchanged from prev
			if prev != nil && y < prev.Height && x < prev.Width {
				pc := prev.Cells[y][x]
				if pc.Rune == cell.Rune && pc.Style == cell.Style {
					continue
				}
			}

			// Move cursor if not sequential
			if y != lastY || x != lastX+1 {
				out.WriteString(CursorTo(x, y))
			}

			// Update style if changed
			if !styleActive || cell.Style != lastStyle {
				out.WriteString(ResetStyle)
				ansi := StyleToANSI(cell.Style)
				if ansi != "" {
					out.WriteString(ansi)
				}
				lastStyle = cell.Style
				styleActive = true
			}

			// Write the rune
			if cell.Rune == 0 {
				out.WriteByte(' ')
			} else {
				var buf [utf8.UTFMax]byte
				n := utf8.EncodeRune(buf[:], cell.Rune)
				out.Write(buf[:n])
			}

			lastX = x
			lastY = y
		}
	}

	if styleActive {
		out.WriteString(ResetStyle)
	}

	return out.String()
}

// FullRender produces ANSI to draw the entire buffer from scratch.
func (b *ScreenBuffer) FullRender() string {
	return b.Diff(nil)
}
