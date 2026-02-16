package tui

import (
	"strings"
	"unicode/utf8"
)

// InputArea is a multi-line text editor component.
type InputArea struct {
	lines    [][]rune // text content, one slice per line
	cursorX  int      // cursor column within current line
	cursorY  int      // cursor line index
	focused  bool
	onSubmit func(string) // called when user presses Enter

	// Visual state
	prompt    string
	hintText  string
	scrollOff int // vertical scroll offset within the input area
}

// NewInputArea creates a new input area.
func NewInputArea() *InputArea {
	return &InputArea{
		lines:    [][]rune{{}},
		prompt:   "> ",
		hintText: "Enter=send  Alt+Enter=newline  Ctrl+C=exit",
	}
}

// Text returns the full text content.
func (ia *InputArea) Text() string {
	var b strings.Builder
	for i, line := range ia.lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(string(line))
	}
	return b.String()
}

// LineCount returns the number of lines of text.
func (ia *InputArea) LineCount() int {
	return len(ia.lines)
}

// Clear resets the input area.
func (ia *InputArea) Clear() {
	ia.lines = [][]rune{{}}
	ia.cursorX = 0
	ia.cursorY = 0
	ia.scrollOff = 0
}

// Focused returns whether the input area has focus.
func (ia *InputArea) Focused() bool { return ia.focused }

// SetFocused sets the focus state.
func (ia *InputArea) SetFocused(f bool) { ia.focused = f }

// MinSize returns the minimum size.
func (ia *InputArea) MinSize() (int, int) { return 10, 3 }

// Update handles keyboard events.
func (ia *InputArea) Update(ev Event) bool {
	if !ia.focused {
		return false
	}

	ke, ok := ev.(KeyEvent)
	if !ok {
		return false
	}

	switch {
	// Submit: Enter (without modifiers)
	case ke.Key == KeyEnter && !ke.Alt && !ke.Ctrl:
		text := ia.Text()
		if strings.TrimSpace(text) == "" {
			return false
		}
		if ia.onSubmit != nil {
			ia.onSubmit(text)
		}
		ia.Clear()
		return true

	// Newline: Alt+Enter
	case ke.Key == KeyEnter && ke.Alt:
		ia.insertNewline()
		return true

	// Backspace
	case ke.Key == KeyBackspace:
		ia.backspace()
		return true

	// Delete
	case ke.Key == KeyDelete:
		ia.delete()
		return true

	// Arrow keys
	case ke.Key == KeyLeft:
		if ke.Alt {
			ia.moveWordLeft()
		} else {
			ia.moveLeft()
		}
		return true
	case ke.Key == KeyRight:
		if ke.Alt {
			ia.moveWordRight()
		} else {
			ia.moveRight()
		}
		return true
	case ke.Key == KeyUp:
		ia.moveUp()
		return true
	case ke.Key == KeyDown:
		ia.moveDown()
		return true

	// Home/End
	case ke.Key == KeyHome:
		ia.cursorX = 0
		return true
	case ke.Key == KeyEnd:
		ia.cursorX = len(ia.lines[ia.cursorY])
		return true

	// Ctrl bindings
	case ke.Ctrl:
		switch ke.Rune {
		case 'a': // beginning of line
			ia.cursorX = 0
			return true
		case 'e': // end of line
			ia.cursorX = len(ia.lines[ia.cursorY])
			return true
		case 'k': // kill to end of line
			ia.lines[ia.cursorY] = ia.lines[ia.cursorY][:ia.cursorX]
			return true
		case 'u': // clear line before cursor
			ia.lines[ia.cursorY] = ia.lines[ia.cursorY][ia.cursorX:]
			ia.cursorX = 0
			return true
		case 'w': // delete word backward
			ia.deleteWordBackward()
			return true
		}
		return false

	// Tab
	case ke.Key == KeyTab:
		// Insert spaces for tab
		for i := 0; i < 4; i++ {
			ia.insertRune(' ')
		}
		return true

	// Printable character
	case ke.Key == KeyRune:
		ia.insertRune(ke.Rune)
		return true
	}

	return false
}

// Render draws the input area.
func (ia *InputArea) Render(buf *ScreenBuffer, bounds Rect) {
	if bounds.Width < 1 || bounds.Height < 1 {
		return
	}

	promptStyle := Style{FG: NewColor(100, 200, 100), Bold: true}
	textStyle := Style{FG: NewColor(220, 220, 220)}
	hintStyle := Style{FG: NewColor(100, 100, 100)}
	cursorStyle := Style{FG: NewColor(0, 0, 0), BG: NewColor(220, 220, 220)}
	borderStyle := Style{FG: NewColor(60, 60, 60)}

	// Draw top border
	for x := bounds.X; x < bounds.X+bounds.Width; x++ {
		buf.Set(x, bounds.Y, '─', borderStyle)
	}

	// Available lines for text (minus border and hint)
	textAreaH := bounds.Height - 2 // border + hint
	if textAreaH < 1 {
		textAreaH = 1
	}

	// Ensure cursor is visible
	if ia.cursorY < ia.scrollOff {
		ia.scrollOff = ia.cursorY
	}
	if ia.cursorY >= ia.scrollOff+textAreaH {
		ia.scrollOff = ia.cursorY - textAreaH + 1
	}

	promptLen := utf8.RuneCountInString(ia.prompt)

	// Draw text lines
	for i := 0; i < textAreaH && i+ia.scrollOff < len(ia.lines); i++ {
		lineIdx := i + ia.scrollOff
		y := bounds.Y + 1 + i
		x := bounds.X

		// Draw prompt on first visible line
		if lineIdx == 0 {
			buf.WriteString(x, y, ia.prompt, promptStyle)
			x += promptLen
		} else {
			// Indent continuation lines to align with prompt
			x += promptLen
		}

		// Draw text
		line := ia.lines[lineIdx]
		availWidth := bounds.X + bounds.Width - x
		for j, r := range line {
			if j >= availWidth {
				break
			}
			if lineIdx == ia.cursorY && j == ia.cursorX && ia.focused {
				buf.Set(x+j, y, r, cursorStyle)
			} else {
				buf.Set(x+j, y, r, textStyle)
			}
		}

		// Draw cursor at end of line
		if lineIdx == ia.cursorY && ia.cursorX >= len(line) && ia.focused {
			cx := x + len(line)
			if cx < bounds.X+bounds.Width {
				buf.Set(cx, y, ' ', cursorStyle)
			}
		}
	}

	// Draw hint line at bottom
	hintY := bounds.Y + bounds.Height - 1
	buf.WriteString(bounds.X+1, hintY, ia.hintText, hintStyle)
}

// --- Editing operations ---

func (ia *InputArea) insertRune(r rune) {
	line := ia.lines[ia.cursorY]
	newLine := make([]rune, len(line)+1)
	copy(newLine, line[:ia.cursorX])
	newLine[ia.cursorX] = r
	copy(newLine[ia.cursorX+1:], line[ia.cursorX:])
	ia.lines[ia.cursorY] = newLine
	ia.cursorX++
}

func (ia *InputArea) insertNewline() {
	line := ia.lines[ia.cursorY]
	before := make([]rune, ia.cursorX)
	copy(before, line[:ia.cursorX])
	after := make([]rune, len(line)-ia.cursorX)
	copy(after, line[ia.cursorX:])

	ia.lines[ia.cursorY] = before

	// Insert new line after current
	newLines := make([][]rune, len(ia.lines)+1)
	copy(newLines, ia.lines[:ia.cursorY+1])
	newLines[ia.cursorY+1] = after
	copy(newLines[ia.cursorY+2:], ia.lines[ia.cursorY+1:])
	ia.lines = newLines

	ia.cursorY++
	ia.cursorX = 0
}

func (ia *InputArea) backspace() {
	if ia.cursorX > 0 {
		line := ia.lines[ia.cursorY]
		ia.lines[ia.cursorY] = append(line[:ia.cursorX-1], line[ia.cursorX:]...)
		ia.cursorX--
	} else if ia.cursorY > 0 {
		// Merge with previous line
		prevLine := ia.lines[ia.cursorY-1]
		currLine := ia.lines[ia.cursorY]
		ia.cursorX = len(prevLine)
		ia.lines[ia.cursorY-1] = append(prevLine, currLine...)

		// Remove current line
		ia.lines = append(ia.lines[:ia.cursorY], ia.lines[ia.cursorY+1:]...)
		ia.cursorY--
	}
}

func (ia *InputArea) delete() {
	line := ia.lines[ia.cursorY]
	if ia.cursorX < len(line) {
		ia.lines[ia.cursorY] = append(line[:ia.cursorX], line[ia.cursorX+1:]...)
	} else if ia.cursorY < len(ia.lines)-1 {
		// Merge with next line
		nextLine := ia.lines[ia.cursorY+1]
		ia.lines[ia.cursorY] = append(line, nextLine...)
		ia.lines = append(ia.lines[:ia.cursorY+1], ia.lines[ia.cursorY+2:]...)
	}
}

func (ia *InputArea) moveLeft() {
	if ia.cursorX > 0 {
		ia.cursorX--
	} else if ia.cursorY > 0 {
		ia.cursorY--
		ia.cursorX = len(ia.lines[ia.cursorY])
	}
}

func (ia *InputArea) moveRight() {
	line := ia.lines[ia.cursorY]
	if ia.cursorX < len(line) {
		ia.cursorX++
	} else if ia.cursorY < len(ia.lines)-1 {
		ia.cursorY++
		ia.cursorX = 0
	}
}

func (ia *InputArea) moveUp() {
	if ia.cursorY > 0 {
		ia.cursorY--
		if ia.cursorX > len(ia.lines[ia.cursorY]) {
			ia.cursorX = len(ia.lines[ia.cursorY])
		}
	}
}

func (ia *InputArea) moveDown() {
	if ia.cursorY < len(ia.lines)-1 {
		ia.cursorY++
		if ia.cursorX > len(ia.lines[ia.cursorY]) {
			ia.cursorX = len(ia.lines[ia.cursorY])
		}
	}
}

func (ia *InputArea) moveWordLeft() {
	if ia.cursorX == 0 {
		ia.moveLeft()
		return
	}
	line := ia.lines[ia.cursorY]
	// Skip whitespace
	for ia.cursorX > 0 && line[ia.cursorX-1] == ' ' {
		ia.cursorX--
	}
	// Skip word characters
	for ia.cursorX > 0 && line[ia.cursorX-1] != ' ' {
		ia.cursorX--
	}
}

func (ia *InputArea) moveWordRight() {
	line := ia.lines[ia.cursorY]
	if ia.cursorX >= len(line) {
		ia.moveRight()
		return
	}
	// Skip word characters
	for ia.cursorX < len(line) && line[ia.cursorX] != ' ' {
		ia.cursorX++
	}
	// Skip whitespace
	for ia.cursorX < len(line) && line[ia.cursorX] == ' ' {
		ia.cursorX++
	}
}

func (ia *InputArea) deleteWordBackward() {
	if ia.cursorX == 0 {
		ia.backspace()
		return
	}
	line := ia.lines[ia.cursorY]
	end := ia.cursorX
	// Skip whitespace
	for ia.cursorX > 0 && line[ia.cursorX-1] == ' ' {
		ia.cursorX--
	}
	// Skip word characters
	for ia.cursorX > 0 && line[ia.cursorX-1] != ' ' {
		ia.cursorX--
	}
	ia.lines[ia.cursorY] = append(line[:ia.cursorX], line[end:]...)
}
