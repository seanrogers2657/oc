package tui

import "testing"

// collectEvents feeds data through parseInput and collects all events.
func collectEvents(data []byte) []Event {
	ch := make(chan Event, 64)
	parseInput(data, ch)
	close(ch)
	var events []Event
	for ev := range ch {
		events = append(events, ev)
	}
	return events
}

func expectKey(t *testing.T, events []Event, idx int, key Key) {
	t.Helper()
	if idx >= len(events) {
		t.Fatalf("expected event at index %d, only got %d events", idx, len(events))
	}
	ke, ok := events[idx].(KeyEvent)
	if !ok {
		t.Fatalf("event %d: expected KeyEvent, got %T", idx, events[idx])
	}
	if ke.Key != key {
		t.Fatalf("event %d: key = %d, want %d", idx, ke.Key, key)
	}
}

func expectRune(t *testing.T, events []Event, idx int, r rune) {
	t.Helper()
	if idx >= len(events) {
		t.Fatalf("expected event at index %d, only got %d events", idx, len(events))
	}
	ke, ok := events[idx].(KeyEvent)
	if !ok {
		t.Fatalf("event %d: expected KeyEvent, got %T", idx, events[idx])
	}
	if ke.Key != KeyRune {
		t.Fatalf("event %d: key = %d, want KeyRune", idx, ke.Key)
	}
	if ke.Rune != r {
		t.Fatalf("event %d: rune = %q, want %q", idx, ke.Rune, r)
	}
}

func expectCtrl(t *testing.T, events []Event, idx int, r rune) {
	t.Helper()
	if idx >= len(events) {
		t.Fatalf("expected event at index %d, only got %d events", idx, len(events))
	}
	ke, ok := events[idx].(KeyEvent)
	if !ok {
		t.Fatalf("event %d: expected KeyEvent, got %T", idx, events[idx])
	}
	if !ke.Ctrl {
		t.Fatalf("event %d: expected Ctrl modifier", idx)
	}
	if ke.Rune != r {
		t.Fatalf("event %d: rune = %q, want %q", idx, ke.Rune, r)
	}
}

func TestInputPrintableASCII(t *testing.T) {
	events := collectEvents([]byte("abc"))
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	expectRune(t, events, 0, 'a')
	expectRune(t, events, 1, 'b')
	expectRune(t, events, 2, 'c')
}

func TestInputUTF8(t *testing.T) {
	events := collectEvents([]byte("héllo"))
	if len(events) != 5 {
		t.Fatalf("expected 5 events, got %d", len(events))
	}
	expectRune(t, events, 0, 'h')
	expectRune(t, events, 1, 'é')
	expectRune(t, events, 2, 'l')
	expectRune(t, events, 3, 'l')
	expectRune(t, events, 4, 'o')
}

func TestInputUTF8Emoji(t *testing.T) {
	events := collectEvents([]byte("🎉"))
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	expectRune(t, events, 0, '🎉')
}

func TestInputEnter(t *testing.T) {
	events := collectEvents([]byte{0x0d}) // CR
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	expectKey(t, events, 0, KeyEnter)

	events = collectEvents([]byte{0x0a}) // LF
	expectKey(t, events, 0, KeyEnter)
}

func TestInputTab(t *testing.T) {
	events := collectEvents([]byte{0x09})
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	expectKey(t, events, 0, KeyTab)
}

func TestInputBackspace(t *testing.T) {
	events := collectEvents([]byte{0x7f})
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	expectKey(t, events, 0, KeyBackspace)

	events = collectEvents([]byte{0x08})
	expectKey(t, events, 0, KeyBackspace)
}

func TestInputCtrlC(t *testing.T) {
	events := collectEvents([]byte{0x03})
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	expectCtrl(t, events, 0, 'c')
}

func TestInputCtrlD(t *testing.T) {
	events := collectEvents([]byte{0x04})
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	expectCtrl(t, events, 0, 'd')
}

func TestInputCtrlA(t *testing.T) {
	events := collectEvents([]byte{0x01})
	expectCtrl(t, events, 0, 'a')
}

func TestInputCtrlE(t *testing.T) {
	events := collectEvents([]byte{0x05})
	expectCtrl(t, events, 0, 'e')
}

func TestInputCtrlK(t *testing.T) {
	events := collectEvents([]byte{0x0b})
	expectCtrl(t, events, 0, 'k')
}

func TestInputCtrlU(t *testing.T) {
	events := collectEvents([]byte{0x15})
	expectCtrl(t, events, 0, 'u')
}

func TestInputCtrlW(t *testing.T) {
	events := collectEvents([]byte{0x17})
	expectCtrl(t, events, 0, 'w')
}

func TestInputCtrlL(t *testing.T) {
	events := collectEvents([]byte{0x0c})
	expectCtrl(t, events, 0, 'l')
}

func TestInputArrowKeys(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		key  Key
	}{
		{"Up", []byte{0x1b, '[', 'A'}, KeyUp},
		{"Down", []byte{0x1b, '[', 'B'}, KeyDown},
		{"Right", []byte{0x1b, '[', 'C'}, KeyRight},
		{"Left", []byte{0x1b, '[', 'D'}, KeyLeft},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := collectEvents(tt.data)
			if len(events) != 1 {
				t.Fatalf("expected 1 event, got %d", len(events))
			}
			expectKey(t, events, 0, tt.key)
		})
	}
}

func TestInputHomeEnd(t *testing.T) {
	// CSI H / CSI F
	events := collectEvents([]byte{0x1b, '[', 'H'})
	expectKey(t, events, 0, KeyHome)

	events = collectEvents([]byte{0x1b, '[', 'F'})
	expectKey(t, events, 0, KeyEnd)

	// SS3 H / SS3 F
	events = collectEvents([]byte{0x1b, 'O', 'H'})
	expectKey(t, events, 0, KeyHome)

	events = collectEvents([]byte{0x1b, 'O', 'F'})
	expectKey(t, events, 0, KeyEnd)
}

func TestInputDelete(t *testing.T) {
	events := collectEvents([]byte{0x1b, '[', '3', '~'})
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	expectKey(t, events, 0, KeyDelete)
}

func TestInputPageUpDown(t *testing.T) {
	events := collectEvents([]byte{0x1b, '[', '5', '~'})
	expectKey(t, events, 0, KeyPgUp)

	events = collectEvents([]byte{0x1b, '[', '6', '~'})
	expectKey(t, events, 0, KeyPgDown)
}

func TestInputEscape(t *testing.T) {
	// Bare escape (single byte, no following sequence data)
	events := collectEvents([]byte{0x1b})
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	expectKey(t, events, 0, KeyEscape)
}

func TestInputAltLetter(t *testing.T) {
	events := collectEvents([]byte{0x1b, 'x'})
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ke := events[0].(KeyEvent)
	if !ke.Alt || ke.Rune != 'x' {
		t.Fatalf("expected Alt+x, got %+v", ke)
	}
}

func TestInputModifiedArrows(t *testing.T) {
	// Ctrl+Up: ESC [ 1 ; 5 A
	events := collectEvents([]byte{0x1b, '[', '1', ';', '5', 'A'})
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ke := events[0].(KeyEvent)
	if ke.Key != KeyUp || !ke.Ctrl {
		t.Fatalf("expected Ctrl+Up, got %+v", ke)
	}

	// Alt+Left: ESC [ 1 ; 3 D
	events = collectEvents([]byte{0x1b, '[', '1', ';', '3', 'D'})
	ke = events[0].(KeyEvent)
	if ke.Key != KeyLeft || !ke.Alt {
		t.Fatalf("expected Alt+Left, got %+v", ke)
	}
}

func TestInputMixed(t *testing.T) {
	// "a" + Up arrow + "b"
	data := []byte{'a', 0x1b, '[', 'A', 'b'}
	events := collectEvents(data)
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	expectRune(t, events, 0, 'a')
	expectKey(t, events, 1, KeyUp)
	expectRune(t, events, 2, 'b')
}

func TestInputBracketedPaste(t *testing.T) {
	// ESC[200~ hello ESC[201~
	data := []byte{0x1b, '[', '2', '0', '0', '~'}
	data = append(data, []byte("hi")...)
	data = append(data, 0x1b, '[', '2', '0', '1', '~')

	events := collectEvents(data)
	if len(events) != 2 {
		t.Fatalf("expected 2 events (h, i), got %d", len(events))
	}
	expectRune(t, events, 0, 'h')
	expectRune(t, events, 1, 'i')
}

// --- SGR Mouse event tests ---

func TestInputMouseScrollUp(t *testing.T) {
	// ESC [ < 64 ; 10 ; 5 M  (scroll up at column 10, row 5)
	data := []byte("\x1b[<64;10;5M")
	events := collectEvents(data)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	se, ok := events[0].(ScrollEvent)
	if !ok {
		t.Fatalf("expected ScrollEvent, got %T", events[0])
	}
	if !se.Up {
		t.Fatal("expected scroll up")
	}
}

func TestInputMouseScrollDown(t *testing.T) {
	data := []byte("\x1b[<65;10;5M")
	events := collectEvents(data)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	se, ok := events[0].(ScrollEvent)
	if !ok {
		t.Fatalf("expected ScrollEvent, got %T", events[0])
	}
	if se.Up {
		t.Fatal("expected scroll down")
	}
}

func TestInputMouseLeftPress(t *testing.T) {
	// ESC [ < 0 ; 15 ; 8 M  (left press at col 15, row 8, 1-based)
	data := []byte("\x1b[<0;15;8M")
	events := collectEvents(data)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	me, ok := events[0].(MouseEvent)
	if !ok {
		t.Fatalf("expected MouseEvent, got %T", events[0])
	}
	if me.Action != MousePress {
		t.Fatalf("expected MousePress, got %d", me.Action)
	}
	// Coordinates should be 0-based (15-1=14, 8-1=7)
	if me.X != 14 || me.Y != 7 {
		t.Fatalf("expected (14, 7), got (%d, %d)", me.X, me.Y)
	}
}

func TestInputMouseLeftDrag(t *testing.T) {
	// ESC [ < 32 ; 20 ; 10 M  (left drag, button 0 + motion flag 32)
	data := []byte("\x1b[<32;20;10M")
	events := collectEvents(data)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	me, ok := events[0].(MouseEvent)
	if !ok {
		t.Fatalf("expected MouseEvent, got %T", events[0])
	}
	if me.Action != MouseDrag {
		t.Fatalf("expected MouseDrag, got %d", me.Action)
	}
	if me.X != 19 || me.Y != 9 {
		t.Fatalf("expected (19, 9), got (%d, %d)", me.X, me.Y)
	}
}

func TestInputMouseLeftRelease(t *testing.T) {
	// ESC [ < 0 ; 25 ; 12 m  (lowercase m = release)
	data := []byte("\x1b[<0;25;12m")
	events := collectEvents(data)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	me, ok := events[0].(MouseEvent)
	if !ok {
		t.Fatalf("expected MouseEvent, got %T", events[0])
	}
	if me.Action != MouseRelease {
		t.Fatalf("expected MouseRelease, got %d", me.Action)
	}
	if me.X != 24 || me.Y != 11 {
		t.Fatalf("expected (24, 11), got (%d, %d)", me.X, me.Y)
	}
}

func TestInputMouseRightButtonIgnored(t *testing.T) {
	// Right press (button 2) should be consumed but produce no event
	data := []byte("\x1b[<2;5;5M")
	events := collectEvents(data)
	if len(events) != 0 {
		t.Fatalf("expected 0 events for right click, got %d", len(events))
	}
}

func TestInputMouseMiddleButtonIgnored(t *testing.T) {
	data := []byte("\x1b[<1;5;5M")
	events := collectEvents(data)
	if len(events) != 0 {
		t.Fatalf("expected 0 events for middle click, got %d", len(events))
	}
}

func TestInputMouseFollowedByText(t *testing.T) {
	// Mouse press then typing 'a'
	data := []byte("\x1b[<0;1;1Ma")
	events := collectEvents(data)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if _, ok := events[0].(MouseEvent); !ok {
		t.Fatalf("expected MouseEvent first, got %T", events[0])
	}
	expectRune(t, events, 1, 'a')
}

func TestInputMouseIncompleteSequence(t *testing.T) {
	// Incomplete SGR sequence (no terminator) — should be treated as bare escape + chars
	data := []byte("\x1b[<0;1;1")
	events := collectEvents(data)
	// parseCSI returns 0 for incomplete, so falls through to bare escape handling
	if len(events) == 0 {
		t.Fatal("expected some events for incomplete sequence")
	}
}

func TestInputBracketedPasteWithNewline(t *testing.T) {
	// Pasted text with newline should send Alt+Enter for literal newline
	data := []byte{0x1b, '[', '2', '0', '0', '~'}
	data = append(data, []byte("a\nb")...)
	data = append(data, 0x1b, '[', '2', '0', '1', '~')

	events := collectEvents(data)
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	expectRune(t, events, 0, 'a')
	// Middle event should be Alt+Enter
	ke := events[1].(KeyEvent)
	if ke.Key != KeyEnter || !ke.Alt {
		t.Fatalf("expected Alt+Enter for pasted newline, got %+v", ke)
	}
	expectRune(t, events, 2, 'b')
}
