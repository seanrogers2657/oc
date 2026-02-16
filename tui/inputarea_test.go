package tui

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
	if ia.LineCount() != 2 {
		t.Fatalf("LineCount() = %d, want 2", ia.LineCount())
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
	if ia.LineCount() != 1 {
		t.Fatalf("LineCount() = %d, want 1", ia.LineCount())
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

func TestInputAreaCtrlK(t *testing.T) {
	ia := newTestInputArea()
	sendRunes(ia, "hello world")
	// Move to position 5
	ia.cursorX = 5
	sendCtrl(ia, 'k')

	if ia.Text() != "hello" {
		t.Fatalf("Ctrl+K: Text() = %q, want %q", ia.Text(), "hello")
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
