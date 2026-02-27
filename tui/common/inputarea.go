package common

import (
	"strings"
	"unicode/utf8"

	"github.com/srogers/oc/domain"
)

// InputArea is a multi-line text editor component.
type InputArea struct {
	lines    [][]rune // text content, one slice per line
	cursorX  int      // cursor column within current line
	cursorY  int      // cursor line index
	focused  bool
	onSubmit func(string) // called when user presses Enter

	// History
	history    []string // previously submitted messages
	historyIdx int      // current position in history (-1 = not browsing)
	draft      string   // saves in-progress text when entering history mode

	// Visual state
	prompt    string
	hintText  string
	scrollOff int // vertical scroll offset within the input area

	// Prompt mode (tool asking user a question)
	promptMode     bool
	promptQuestion string
}

// NewInputArea creates a new input area.
func NewInputArea() *InputArea {
	return &InputArea{
		lines:      [][]rune{{}},
		historyIdx: -1,
		prompt:     "> ",
		hintText:   "Enter=send  Alt+Enter=newline  : /=commands  C-j/k=scroll  C-c=exit",
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

// visualLine maps a segment of a logical line to a visual (screen) row.
type visualLine struct {
	logicalLine int // index into ia.lines
	startCol    int // start rune offset within the logical line
	length      int // number of runes in this visual segment
}

// computeVisualLines splits each logical line into segments of availWidth runes.
// Empty logical lines produce one visual line with length 0.
func (ia *InputArea) computeVisualLines(availWidth int) []visualLine {
	if availWidth <= 0 {
		availWidth = 1
	}
	var vlines []visualLine
	for i, line := range ia.lines {
		if len(line) == 0 {
			vlines = append(vlines, visualLine{logicalLine: i, startCol: 0, length: 0})
			continue
		}
		for off := 0; off < len(line); off += availWidth {
			end := off + availWidth
			if end > len(line) {
				end = len(line)
			}
			vlines = append(vlines, visualLine{logicalLine: i, startCol: off, length: end - off})
		}
	}
	return vlines
}

// cursorVisualPos maps (ia.cursorY, ia.cursorX) to a visual row and column.
func (ia *InputArea) cursorVisualPos(vlines []visualLine, availWidth int) (vRow, vCol int) {
	if availWidth <= 0 {
		availWidth = 1
	}
	for i, vl := range vlines {
		if vl.logicalLine != ia.cursorY {
			continue
		}
		// Cursor is in this logical line — check if it falls within this segment.
		if ia.cursorX >= vl.startCol && ia.cursorX < vl.startCol+vl.length {
			return i, ia.cursorX - vl.startCol
		}
		// Cursor is at end of segment (on the boundary):
		// It belongs to this segment if it's at the end of the logical line,
		// or if it equals startCol+length and this is the last segment of the line.
		if ia.cursorX == vl.startCol+vl.length {
			// Check if there's a next segment for the same logical line.
			if i+1 < len(vlines) && vlines[i+1].logicalLine == ia.cursorY {
				// Cursor belongs to the next segment (col 0).
				continue
			}
			// Last segment of this logical line — cursor is at end.
			return i, ia.cursorX - vl.startCol
		}
	}
	// Fallback: last visual line.
	if len(vlines) > 0 {
		return len(vlines) - 1, 0
	}
	return 0, 0
}

// LineCount returns the number of visual lines of text (including wrapped question lines in prompt mode).
// width is the available rendering width (used to calculate question and text wrapping).
func (ia *InputArea) LineCount(width int) int {
	promptLen := utf8.RuneCountInString(ia.prompt)
	availWidth := width - promptLen
	if availWidth <= 0 {
		availWidth = 1
	}
	n := len(ia.computeVisualLines(availWidth))
	if ia.promptMode && ia.promptQuestion != "" {
		n += ia.questionLineCount(width)
	}
	return n
}

// questionLineCount returns how many lines the prompt question occupies when wrapped to fit.
func (ia *InputArea) questionLineCount(width int) int {
	if ia.promptQuestion == "" {
		return 0
	}
	// Available width: total width minus 1 for left margin
	availWidth := width - 1
	if availWidth <= 0 {
		availWidth = 1
	}
	lines := WrapText(ia.promptQuestion, availWidth)
	if len(lines) == 0 {
		return 1
	}
	return len(lines)
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
func (ia *InputArea) MinSize() (int, int) { return 10, 4 }

// Update handles keyboard events. It resolves the key to an action and
// delegates to HandleAction / InsertRune.
func (ia *InputArea) Update(ev Event) bool {
	ke, ok := ev.(domain.KeyEvent)
	if !ok || !ia.focused {
		return false
	}
	if action, ok := resolveInputAction(ke); ok {
		return ia.HandleAction(action)
	}
	if ke.Key == domain.KeyRune && ke.Mod == 0 {
		return ia.InsertRune(ke.Rune)
	}
	return false
}

// resolveInputAction maps a key event to an input action using the same
// hardcoded rules that existed before the KeyMap system. This is used by
// Update() for backward compatibility when no KeyMap is present.
func resolveInputAction(ke domain.KeyEvent) (domain.Action, bool) {
	switch {
	case ke.Key == domain.KeyEnter && ke.Mod == 0:
		return domain.ActionSubmit, true
	case ke.Key == domain.KeyEnter && ke.Mod.Has(domain.ModAlt):
		return domain.ActionNewline, true
	case ke.Key == domain.KeyBackspace && ke.Mod.Has(domain.ModAlt):
		return domain.ActionDeleteWordBackward, true
	case ke.Key == domain.KeyBackspace:
		return domain.ActionBackspace, true
	case ke.Key == domain.KeyDelete:
		return domain.ActionDelete, true
	case ke.Key == domain.KeyLeft && ke.Mod.Has(domain.ModAlt):
		return domain.ActionWordLeft, true
	case ke.Key == domain.KeyLeft:
		return domain.ActionCursorLeft, true
	case ke.Key == domain.KeyRight && ke.Mod.Has(domain.ModAlt):
		return domain.ActionWordRight, true
	case ke.Key == domain.KeyRight:
		return domain.ActionCursorRight, true
	case ke.Key == domain.KeyUp:
		return domain.ActionCursorUp, true
	case ke.Key == domain.KeyDown:
		return domain.ActionCursorDown, true
	case ke.Key == domain.KeyHome:
		return domain.ActionHome, true
	case ke.Key == domain.KeyEnd:
		return domain.ActionEnd, true
	case ke.Key == domain.KeyEscape:
		return domain.ActionClearInput, true
	case ke.Key == domain.KeyRune && ke.Mod.Has(domain.ModAlt) && ke.Rune == 'b':
		return domain.ActionWordLeft, true
	case ke.Key == domain.KeyRune && ke.Mod.Has(domain.ModAlt) && ke.Rune == 'f':
		return domain.ActionWordRight, true
	case ke.Mod.Has(domain.ModCtrl) && ke.Rune == 'a':
		return domain.ActionHome, true
	case ke.Mod.Has(domain.ModCtrl) && ke.Rune == 'e':
		return domain.ActionEnd, true
	case ke.Mod.Has(domain.ModCtrl) && ke.Rune == 'u':
		return domain.ActionKillLineBefore, true
	case ke.Mod.Has(domain.ModCtrl) && ke.Rune == 'w':
		return domain.ActionDeleteWordBackward, true
	case ke.Key == domain.KeyTab:
		return domain.ActionInsertTab, true
	}
	return "", false
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

	// In prompt mode, show the question on the lines after the border (word-wrapped)
	questionLines := 0
	if ia.promptMode && ia.promptQuestion != "" {
		questionStyle := Style{FG: NewColor(255, 200, 60), Bold: true}
		availWidth := bounds.Width - 1 // 1 for left margin
		if availWidth <= 0 {
			availWidth = 1
		}
		wrapped := WrapText(ia.promptQuestion, availWidth)
		for i, line := range wrapped {
			qy := bounds.Y + 1 + i
			if qy >= bounds.Y+bounds.Height {
				break
			}
			buf.WriteString(bounds.X+1, qy, line, questionStyle)
		}
		questionLines = len(wrapped)
	}

	// Available lines for text (minus top border, question, bottom border, and hint)
	textAreaH := bounds.Height - 3 - questionLines // top border + bottom border + hint + question
	if textAreaH < 1 {
		textAreaH = 1
	}

	promptLen := utf8.RuneCountInString(ia.prompt)
	availWidth := bounds.Width - promptLen
	if availWidth <= 0 {
		availWidth = 1
	}

	// Compute visual lines and cursor position
	vlines := ia.computeVisualLines(availWidth)
	cursorVRow, cursorVCol := ia.cursorVisualPos(vlines, availWidth)

	// Ensure cursor is visible (scroll in visual rows)
	if cursorVRow < ia.scrollOff {
		ia.scrollOff = cursorVRow
	}
	if cursorVRow >= ia.scrollOff+textAreaH {
		ia.scrollOff = cursorVRow - textAreaH + 1
	}

	// Draw visual lines
	for i := 0; i < textAreaH && i+ia.scrollOff < len(vlines); i++ {
		vl := vlines[i+ia.scrollOff]
		y := bounds.Y + 1 + questionLines + i
		x := bounds.X

		// Draw prompt on the very first visual line
		if i+ia.scrollOff == 0 {
			buf.WriteString(x, y, ia.prompt, promptStyle)
		}
		x += promptLen

		// Draw text segment
		line := ia.lines[vl.logicalLine]
		for j := 0; j < vl.length; j++ {
			r := line[vl.startCol+j]
			if i+ia.scrollOff == cursorVRow && j == cursorVCol && ia.focused {
				buf.Set(x+j, y, r, cursorStyle)
			} else {
				buf.Set(x+j, y, r, textStyle)
			}
		}

		// Draw cursor at end of segment (when cursor is past the last char)
		if i+ia.scrollOff == cursorVRow && cursorVCol >= vl.length && ia.focused {
			cx := x + vl.length
			if cx < bounds.X+bounds.Width {
				buf.Set(cx, y, ' ', cursorStyle)
			}
		}
	}

	// Draw separator above hint
	sepY := bounds.Y + bounds.Height - 2
	for x := bounds.X; x < bounds.X+bounds.Width; x++ {
		buf.Set(x, sepY, '─', borderStyle)
	}

	// Draw hint line at bottom
	hintY := bounds.Y + bounds.Height - 1
	buf.WriteString(bounds.X+1, hintY, ia.hintText, hintStyle)
}

// SetPromptMode switches the input area into or out of prompt mode.
func (ia *InputArea) SetPromptMode(on bool, question string) {
	ia.promptMode = on
	ia.promptQuestion = question
	ia.Clear()
	if on {
		ia.prompt = "? "
		ia.hintText = "Enter=answer"
	} else {
		ia.prompt = "> "
		ia.hintText = "Enter=send  Alt+Enter=newline  : /=commands  C-j/k=scroll  C-c=exit"
	}
}

// SetOnSubmit sets the callback invoked when the user presses Enter.
func (ia *InputArea) SetOnSubmit(fn func(string)) {
	ia.onSubmit = fn
}

// InPromptMode returns whether the input area is in prompt mode.
func (ia *InputArea) InPromptMode() bool {
	return ia.promptMode
}

// HandleAction responds to a resolved action. Returns true if the component needs redraw.
func (ia *InputArea) HandleAction(action domain.Action) bool {
	if !ia.focused {
		return false
	}
	switch action {
	case domain.ActionSubmit:
		text := ia.Text()
		if strings.TrimSpace(text) == "" {
			return false
		}
		ia.history = append(ia.history, text)
		ia.historyIdx = -1
		ia.draft = ""
		if ia.onSubmit != nil {
			ia.onSubmit(text)
		}
		ia.Clear()
		return true
	case domain.ActionNewline:
		ia.insertNewline()
		return true
	case domain.ActionDeleteWordBackward:
		ia.deleteWordBackward()
		return true
	case domain.ActionBackspace:
		ia.backspace()
		return true
	case domain.ActionDelete:
		ia.delete()
		return true
	case domain.ActionCursorLeft:
		ia.moveLeft()
		return true
	case domain.ActionCursorRight:
		ia.moveRight()
		return true
	case domain.ActionWordLeft:
		ia.moveWordLeft()
		return true
	case domain.ActionWordRight:
		ia.moveWordRight()
		return true
	case domain.ActionCursorUp:
		if ia.cursorY == 0 {
			ia.historyBack()
		} else {
			ia.moveUp()
		}
		return true
	case domain.ActionCursorDown:
		if ia.cursorY == len(ia.lines)-1 {
			ia.historyForward()
		} else {
			ia.moveDown()
		}
		return true
	case domain.ActionHome:
		ia.cursorX = 0
		return true
	case domain.ActionEnd:
		ia.cursorX = len(ia.lines[ia.cursorY])
		return true
	case domain.ActionKillLineBefore:
		ia.lines[ia.cursorY] = ia.lines[ia.cursorY][ia.cursorX:]
		ia.cursorX = 0
		return true
	case domain.ActionClearInput:
		ia.Clear()
		return true
	case domain.ActionInsertTab:
		for i := 0; i < 4; i++ {
			ia.insertRune(' ')
		}
		return true
	}
	return false
}

// InsertRune handles a typed character not bound to any action. Returns true if redraw needed.
func (ia *InputArea) InsertRune(r rune) bool {
	if !ia.focused {
		return false
	}
	ia.insertRune(r)
	return true
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

// SetHistory replaces the history slice (used at startup to load persisted history).
func (ia *InputArea) SetHistory(entries []string) {
	ia.history = make([]string, len(entries))
	copy(ia.history, entries)
	ia.historyIdx = -1
}

// History returns a copy of the history slice (used at shutdown to save).
func (ia *InputArea) History() []string {
	out := make([]string, len(ia.history))
	copy(out, ia.history)
	return out
}

// --- History ---

func (ia *InputArea) historyBack() {
	if len(ia.history) == 0 {
		return
	}
	if ia.historyIdx == -1 {
		ia.draft = ia.Text()
		ia.historyIdx = len(ia.history) - 1
	} else if ia.historyIdx > 0 {
		ia.historyIdx--
	}
	ia.loadText(ia.history[ia.historyIdx])
}

func (ia *InputArea) historyForward() {
	if ia.historyIdx == -1 {
		return
	}
	ia.historyIdx++
	if ia.historyIdx >= len(ia.history) {
		ia.historyIdx = -1
		ia.loadText(ia.draft)
		ia.draft = ""
	} else {
		ia.loadText(ia.history[ia.historyIdx])
	}
}

func (ia *InputArea) loadText(s string) {
	parts := strings.Split(s, "\n")
	ia.lines = make([][]rune, len(parts))
	for i, p := range parts {
		ia.lines[i] = []rune(p)
	}
	ia.cursorY = len(ia.lines) - 1
	ia.cursorX = len(ia.lines[ia.cursorY])
	ia.scrollOff = 0
}
