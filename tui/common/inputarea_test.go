package common

import "testing"

func newTestInputArea() *InputArea {
	ia := NewInputArea()
	ia.SetFocused(true)
	return ia
}

func sendRunes(ia *InputArea, s string) {
	for _, r := range s {
		ia.Update(KeyEvent{Key: KeyRune, Rune: r})
	}
}

func sendKey(ia *InputArea, key Key) {
	ia.Update(KeyEvent{Key: key})
}

func sendCtrl(ia *InputArea, r rune) {
	ia.Update(KeyEvent{Key: KeyRune, Rune: r, Ctrl: true})
}

func TestInputAreaTyping(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "hello")

	if ia.Text() != "hello" {
		t.Fatalf("Text() = %q, want %q", ia.Text(), "hello")
	}
	if ia.cursorX != 5 {
		t.Fatalf("cursorX = %d, want 5", ia.cursorX)
	}
}

func TestInputAreaNewline(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "line1")
	ia.Update(KeyEvent{Key: KeyEnter, Alt: true}) // Alt+Enter = newline
	sendRunes(ia, "line2")

	if ia.Text() != "line1\nline2" {
		t.Fatalf("Text() = %q, want %q", ia.Text(), "line1\nline2")
	}
	if ia.LineCount(80) != 2 {
		t.Fatalf("LineCount() = %d, want 2", ia.LineCount(80))
	}
}

func TestInputAreaSubmit(t *testing.T) {
	ia := newTestInputArea()
	var submitted string
	ia.onSubmit = func(text string) { submitted = text }

	sendRunes(ia, "hello world")
	sendKey(ia, KeyEnter)

	if submitted != "hello world" {
		t.Fatalf("submitted = %q, want %q", submitted, "hello world")
	}
	// Input should be cleared after submit
	if ia.Text() != "" {
		t.Fatalf("Text() after submit = %q, want empty", ia.Text())
	}
}

func TestInputAreaSubmitEmpty(t *testing.T) {
	ia := newTestInputArea()
	called := false
	ia.onSubmit = func(text string) { called = true }

	sendKey(ia, KeyEnter) // empty input
	if called {
		t.Fatal("onSubmit should not be called for empty input")
	}
}

func TestInputAreaSubmitWhitespace(t *testing.T) {
	ia := newTestInputArea()
	called := false
	ia.onSubmit = func(text string) { called = true }

	sendRunes(ia, "   ")
	sendKey(ia, KeyEnter)
	if called {
		t.Fatal("onSubmit should not be called for whitespace-only input")
	}
}

func TestInputAreaBackspace(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "hello")
	sendKey(ia, KeyBackspace)

	if ia.Text() != "hell" {
		t.Fatalf("Text() = %q, want %q", ia.Text(), "hell")
	}
}

func TestInputAreaBackspaceAtStart(t *testing.T) {
	ia := newTestInputArea()
	sendKey(ia, KeyBackspace) // should not panic
	if ia.Text() != "" {
		t.Fatalf("Text() = %q, want empty", ia.Text())
	}
}

func TestInputAreaBackspaceMergeLines(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "hello")
	ia.Update(KeyEvent{Key: KeyEnter, Alt: true})
	sendRunes(ia, "world")

	// Move to beginning of line 2
	sendCtrl(ia, 'a')
	sendKey(ia, KeyBackspace) // merge with line 1

	if ia.Text() != "helloworld" {
		t.Fatalf("Text() = %q, want %q", ia.Text(), "helloworld")
	}
	if ia.LineCount(80) != 1 {
		t.Fatalf("LineCount() = %d, want 1", ia.LineCount(80))
	}
}

func TestInputAreaDelete(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "hello")
	sendCtrl(ia, 'a') // move to start
	sendKey(ia, KeyDelete)

	if ia.Text() != "ello" {
		t.Fatalf("Text() = %q, want %q", ia.Text(), "ello")
	}
}

func TestInputAreaDeleteMergeLines(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "hello")
	ia.Update(KeyEvent{Key: KeyEnter, Alt: true})
	sendRunes(ia, "world")

	// Move to end of line 1
	ia.cursorY = 0
	sendCtrl(ia, 'e')
	sendKey(ia, KeyDelete) // merge with line 2

	if ia.Text() != "helloworld" {
		t.Fatalf("Text() = %q, want %q", ia.Text(), "helloworld")
	}
}

func TestInputAreaArrowLeftRight(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "abc")

	sendKey(ia, KeyLeft)
	if ia.cursorX != 2 {
		t.Fatalf("cursorX = %d, want 2", ia.cursorX)
	}

	sendKey(ia, KeyRight)
	if ia.cursorX != 3 {
		t.Fatalf("cursorX = %d, want 3", ia.cursorX)
	}
}

func TestInputAreaArrowLeftWrapToPrevLine(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "abc")
	ia.Update(KeyEvent{Key: KeyEnter, Alt: true})
	sendRunes(ia, "de")

	// At start of line 2, go left => end of line 1
	sendCtrl(ia, 'a')
	sendKey(ia, KeyLeft)
	if ia.cursorY != 0 || ia.cursorX != 3 {
		t.Fatalf("cursor = (%d,%d), want (3,0)", ia.cursorX, ia.cursorY)
	}
}

func TestInputAreaArrowUpDown(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "line1")
	ia.Update(KeyEvent{Key: KeyEnter, Alt: true})
	sendRunes(ia, "line2")

	sendKey(ia, KeyUp)
	if ia.cursorY != 0 {
		t.Fatalf("cursorY = %d, want 0", ia.cursorY)
	}

	sendKey(ia, KeyDown)
	if ia.cursorY != 1 {
		t.Fatalf("cursorY = %d, want 1", ia.cursorY)
	}
}

func TestInputAreaHomeEnd(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "hello")

	sendKey(ia, KeyHome)
	if ia.cursorX != 0 {
		t.Fatalf("Home: cursorX = %d, want 0", ia.cursorX)
	}

	sendKey(ia, KeyEnd)
	if ia.cursorX != 5 {
		t.Fatalf("End: cursorX = %d, want 5", ia.cursorX)
	}
}

func TestInputAreaCtrlA(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "hello")
	sendCtrl(ia, 'a')
	if ia.cursorX != 0 {
		t.Fatalf("Ctrl+A: cursorX = %d, want 0", ia.cursorX)
	}
}

func TestInputAreaCtrlE(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "hello")
	sendCtrl(ia, 'a')
	sendCtrl(ia, 'e')
	if ia.cursorX != 5 {
		t.Fatalf("Ctrl+E: cursorX = %d, want 5", ia.cursorX)
	}
}

func TestInputAreaCtrlU(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "hello world")
	ia.cursorX = 6
	sendCtrl(ia, 'u')

	if ia.Text() != "world" {
		t.Fatalf("Ctrl+U: Text() = %q, want %q", ia.Text(), "world")
	}
	if ia.cursorX != 0 {
		t.Fatalf("Ctrl+U: cursorX = %d, want 0", ia.cursorX)
	}
}

func TestInputAreaCtrlW(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "hello world")
	sendCtrl(ia, 'w')

	if ia.Text() != "hello " {
		t.Fatalf("Ctrl+W: Text() = %q, want %q", ia.Text(), "hello ")
	}
}

func TestInputAreaCtrlWMultipleWords(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "one two three")
	sendCtrl(ia, 'w')
	if ia.Text() != "one two " {
		t.Fatalf("Ctrl+W: Text() = %q, want %q", ia.Text(), "one two ")
	}
	sendCtrl(ia, 'w')
	if ia.Text() != "one " {
		t.Fatalf("Ctrl+W x2: Text() = %q, want %q", ia.Text(), "one ")
	}
}

func TestInputAreaWordMovement(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "hello world foo")

	// Alt+Left: move word left
	ia.Update(KeyEvent{Key: KeyLeft, Alt: true})
	if ia.cursorX != 12 {
		t.Fatalf("Alt+Left: cursorX = %d, want 12", ia.cursorX)
	}

	ia.Update(KeyEvent{Key: KeyLeft, Alt: true})
	if ia.cursorX != 6 {
		t.Fatalf("Alt+Left x2: cursorX = %d, want 6", ia.cursorX)
	}

	// Alt+Right: move word right
	ia.Update(KeyEvent{Key: KeyRight, Alt: true})
	if ia.cursorX != 12 {
		t.Fatalf("Alt+Right: cursorX = %d, want 12", ia.cursorX)
	}
}

func TestInputAreaAltB(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "hello world")
	ia.Update(KeyEvent{Key: KeyRune, Rune: 'b', Alt: true})
	if ia.cursorX != 6 {
		t.Fatalf("Alt+B: cursorX = %d, want 6", ia.cursorX)
	}
	ia.Update(KeyEvent{Key: KeyRune, Rune: 'b', Alt: true})
	if ia.cursorX != 0 {
		t.Fatalf("Alt+B x2: cursorX = %d, want 0", ia.cursorX)
	}
}

func TestInputAreaAltF(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "hello world")
	ia.cursorX = 0
	ia.Update(KeyEvent{Key: KeyRune, Rune: 'f', Alt: true})
	if ia.cursorX != 6 {
		t.Fatalf("Alt+F: cursorX = %d, want 6", ia.cursorX)
	}
	ia.Update(KeyEvent{Key: KeyRune, Rune: 'f', Alt: true})
	if ia.cursorX != 11 {
		t.Fatalf("Alt+F x2: cursorX = %d, want 11", ia.cursorX)
	}
}

func TestInputAreaEscapeClearsInput(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "hello world")
	ia.Update(KeyEvent{Key: KeyEscape})
	if ia.Text() != "" {
		t.Fatalf("Escape: Text() = %q, want empty", ia.Text())
	}
	if ia.cursorX != 0 || ia.cursorY != 0 {
		t.Fatalf("Escape: cursor = (%d,%d), want (0,0)", ia.cursorX, ia.cursorY)
	}
}

func TestInputAreaAltBackspace(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "hello world")
	ia.Update(KeyEvent{Key: KeyBackspace, Alt: true})
	if ia.Text() != "hello " {
		t.Fatalf("Alt+Backspace: Text() = %q, want %q", ia.Text(), "hello ")
	}
}

func TestInputAreaTab(t *testing.T) {
	ia := newTestInputArea()
	sendKey(ia, KeyTab)
	if ia.Text() != "    " {
		t.Fatalf("Tab: Text() = %q, want 4 spaces", ia.Text())
	}
}

func TestInputAreaNotFocused(t *testing.T) {
	ia := NewInputArea()
	ia.SetFocused(false)
	dirty := ia.Update(KeyEvent{Key: KeyRune, Rune: 'a'})
	if dirty {
		t.Fatal("unfocused input area should not process events")
	}
	if ia.Text() != "" {
		t.Fatal("unfocused input area should not accept input")
	}
}

func TestInputAreaRender(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "hello")

	buf := NewScreenBuffer(40, 5)
	ia.Render(buf, Rect{X: 0, Y: 0, Width: 40, Height: 5})

	// Should see the prompt "> " and "hello" on row 1 (row 0 is border)
	// Border on row 0
	if buf.Cells[0][0].Rune != '─' {
		t.Fatalf("row 0 should be border, got %q", buf.Cells[0][0].Rune)
	}

	// Prompt on row 1
	if buf.Cells[1][0].Rune != '>' {
		t.Fatalf("row 1 col 0 = %q, want '>'", buf.Cells[1][0].Rune)
	}

	// Text starts at prompt offset
	if buf.Cells[1][2].Rune != 'h' {
		t.Fatalf("row 1 col 2 = %q, want 'h'", buf.Cells[1][2].Rune)
	}
}

func TestInputAreaInsertInMiddle(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "hllo")
	ia.cursorX = 1
	ia.Update(KeyEvent{Key: KeyRune, Rune: 'e'})

	if ia.Text() != "hello" {
		t.Fatalf("Text() = %q, want %q", ia.Text(), "hello")
	}
	if ia.cursorX != 2 {
		t.Fatalf("cursorX = %d, want 2", ia.cursorX)
	}
}

func TestInputAreaNewlineInMiddle(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "helloworld")
	ia.cursorX = 5
	ia.Update(KeyEvent{Key: KeyEnter, Alt: true})

	if ia.Text() != "hello\nworld" {
		t.Fatalf("Text() = %q, want %q", ia.Text(), "hello\nworld")
	}
	if ia.cursorY != 1 || ia.cursorX != 0 {
		t.Fatalf("cursor = (%d,%d), want (0,1)", ia.cursorX, ia.cursorY)
	}
}

// submitText is a helper that types text and presses Enter to submit it.
func submitText(ia *InputArea, s string) {
	sendRunes(ia, s)
	sendKey(ia, KeyEnter)
}

func TestInputAreaHistoryUp(t *testing.T) {
	ia := newTestInputArea()
	ia.onSubmit = func(string) {}

	submitText(ia, "first")
	submitText(ia, "second")
	submitText(ia, "third")

	sendKey(ia, KeyUp) // -> "third"
	if ia.Text() != "third" {
		t.Fatalf("Up 1: Text() = %q, want %q", ia.Text(), "third")
	}
	sendKey(ia, KeyUp) // -> "second"
	if ia.Text() != "second" {
		t.Fatalf("Up 2: Text() = %q, want %q", ia.Text(), "second")
	}
	sendKey(ia, KeyUp) // -> "first"
	if ia.Text() != "first" {
		t.Fatalf("Up 3: Text() = %q, want %q", ia.Text(), "first")
	}
}

func TestInputAreaHistoryDown(t *testing.T) {
	ia := newTestInputArea()
	ia.onSubmit = func(string) {}

	submitText(ia, "first")
	submitText(ia, "second")

	sendKey(ia, KeyUp) // -> "second"
	sendKey(ia, KeyUp) // -> "first"
	sendKey(ia, KeyDown) // -> "second"
	if ia.Text() != "second" {
		t.Fatalf("Down: Text() = %q, want %q", ia.Text(), "second")
	}
	sendKey(ia, KeyDown) // -> back to empty (draft)
	if ia.Text() != "" {
		t.Fatalf("Down past end: Text() = %q, want empty", ia.Text())
	}
}

func TestInputAreaHistoryDraftPreserved(t *testing.T) {
	ia := newTestInputArea()
	ia.onSubmit = func(string) {}

	submitText(ia, "old message")

	// Type partial text without submitting
	sendRunes(ia, "work in progress")
	sendKey(ia, KeyUp) // -> "old message"
	if ia.Text() != "old message" {
		t.Fatalf("Up: Text() = %q, want %q", ia.Text(), "old message")
	}
	sendKey(ia, KeyDown) // -> restore draft
	if ia.Text() != "work in progress" {
		t.Fatalf("Down (draft): Text() = %q, want %q", ia.Text(), "work in progress")
	}
}

func TestInputAreaHistoryUpAtBound(t *testing.T) {
	ia := newTestInputArea()
	ia.onSubmit = func(string) {}

	submitText(ia, "only")

	sendKey(ia, KeyUp) // -> "only"
	sendKey(ia, KeyUp) // -> still "only" (clamped)
	sendKey(ia, KeyUp) // -> still "only"
	if ia.Text() != "only" {
		t.Fatalf("Up at bound: Text() = %q, want %q", ia.Text(), "only")
	}
}

func TestInputAreaPromptModeShortQuestion(t *testing.T) {
	ia := newTestInputArea()
	ia.SetPromptMode(true, "Allow this action?")

	// Short question fits on one line at width 80
	if ia.LineCount(80) != 2 { // 1 question line + 1 text line
		t.Fatalf("LineCount(80) = %d, want 2", ia.LineCount(80))
	}
	if !ia.promptMode {
		t.Fatal("expected promptMode to be true")
	}
	if ia.prompt != "? " {
		t.Fatalf("prompt = %q, want %q", ia.prompt, "? ")
	}
}

func TestInputAreaPromptModeLongQuestionWraps(t *testing.T) {
	ia := newTestInputArea()
	// Question longer than 40 chars should wrap at width 40
	question := "Do you want to allow this tool to execute the dangerous bash command?"
	ia.SetPromptMode(true, question)

	// Available width = 40 - 1 (left margin) = 39
	// 68-char question wraps to 2 lines at width 39
	lineCount := ia.LineCount(40)
	if lineCount < 3 { // at least 2 question lines + 1 text line
		t.Fatalf("LineCount(40) = %d, want >= 3 (question should wrap)", lineCount)
	}

	// At width 80 it should fit in fewer lines
	wideCount := ia.LineCount(80)
	if wideCount >= lineCount {
		t.Fatalf("LineCount(80) = %d should be less than LineCount(40) = %d", wideCount, lineCount)
	}
}

func TestInputAreaPromptModeRenderWrapsQuestion(t *testing.T) {
	ia := newTestInputArea()
	question := "This is a long question that should wrap across multiple lines"
	ia.SetPromptMode(true, question)

	// Render into a narrow buffer (30 wide)
	buf := NewScreenBuffer(30, 10)
	ia.Render(buf, Rect{X: 0, Y: 0, Width: 30, Height: 10})

	// Row 0: border
	if buf.Cells[0][0].Rune != '─' {
		t.Fatalf("row 0 should be border, got %q", buf.Cells[0][0].Rune)
	}

	// Row 1: first line of question should start at col 1 (left margin)
	if buf.Cells[1][1].Rune != 'T' {
		t.Fatalf("row 1 col 1 = %q, want 'T' (start of question)", buf.Cells[1][1].Rune)
	}

	// Row 2: second wrapped line of question should also have content
	// Available width = 30 - 1 = 29 chars per line
	// "This is a long question that " = 29 chars on line 1
	// "should wrap across multiple l" = 29 chars on line 2
	// "ines" = 4 chars on line 3
	hasContent := false
	for x := 1; x < 30; x++ {
		if buf.Cells[2][x].Rune != 0 {
			hasContent = true
			break
		}
	}
	if !hasContent {
		t.Fatal("row 2 should contain wrapped question text")
	}
}

func TestInputAreaPromptModeExitRestoresState(t *testing.T) {
	ia := newTestInputArea()
	ia.SetPromptMode(true, "Allow?")

	if ia.prompt != "? " {
		t.Fatalf("prompt in prompt mode = %q, want %q", ia.prompt, "? ")
	}
	if ia.hintText != "Enter=answer" {
		t.Fatalf("hintText in prompt mode = %q, want %q", ia.hintText, "Enter=answer")
	}

	ia.SetPromptMode(false, "")

	if ia.promptMode {
		t.Fatal("expected promptMode to be false after exit")
	}
	if ia.prompt != "> " {
		t.Fatalf("prompt after exit = %q, want %q", ia.prompt, "> ")
	}
	// LineCount should not include question lines
	if ia.LineCount(80) != 1 {
		t.Fatalf("LineCount(80) after exit = %d, want 1", ia.LineCount(80))
	}
}

func TestInputAreaPromptModeEmptyQuestion(t *testing.T) {
	ia := newTestInputArea()
	ia.SetPromptMode(true, "")

	// Empty question should not add extra lines
	if ia.LineCount(80) != 1 {
		t.Fatalf("LineCount(80) with empty question = %d, want 1", ia.LineCount(80))
	}
}

func TestInputAreaHistoryMultiLineUpDown(t *testing.T) {
	ia := newTestInputArea()
	ia.onSubmit = func(string) {}

	submitText(ia, "previous")

	// Type multi-line input
	sendRunes(ia, "line1")
	ia.Update(KeyEvent{Key: KeyEnter, Alt: true}) // newline
	sendRunes(ia, "line2")

	// Cursor is on line 2 (last line). Up should move to line 1, not history.
	sendKey(ia, KeyUp)
	if ia.cursorY != 0 {
		t.Fatalf("Up from line2: cursorY = %d, want 0", ia.cursorY)
	}
	if ia.Text() != "line1\nline2" {
		t.Fatalf("Up from line2: Text() = %q, want %q", ia.Text(), "line1\nline2")
	}

	// Now on line 1 (first line). Up should enter history.
	sendKey(ia, KeyUp)
	if ia.Text() != "previous" {
		t.Fatalf("Up from line1: Text() = %q, want %q", ia.Text(), "previous")
	}

	// Down should restore draft (multi-line)
	sendKey(ia, KeyDown)
	if ia.Text() != "line1\nline2" {
		t.Fatalf("Down to draft: Text() = %q, want %q", ia.Text(), "line1\nline2")
	}

	// Cursor should be on last line. Down should be a no-op (already at last line, not in history).
	sendKey(ia, KeyDown)
	if ia.Text() != "line1\nline2" {
		t.Fatalf("Down at bottom: Text() = %q, want %q", ia.Text(), "line1\nline2")
	}
}
