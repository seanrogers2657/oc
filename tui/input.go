package tui

import (
	"io"
	"unicode/utf8"
)

// ReadInput reads raw bytes from r and sends parsed KeyEvents to the events channel.
// Runs until r returns an error (typically when stdin is closed).
// Expects the terminal to be in raw mode.
func ReadInput(r io.Reader, events chan<- Event) {
	buf := make([]byte, 256)
	for {
		n, err := r.Read(buf)
		if err != nil {
			return
		}
		parseInput(buf[:n], events)
	}
}

func parseInput(data []byte, events chan<- Event) {
	i := 0
	for i < len(data) {
		// ESC sequence
		if data[i] == 0x1b {
			consumed := parseEscape(data[i:], events)
			if consumed > 0 {
				i += consumed
				continue
			}
			// Bare escape
			events <- KeyEvent{Key: KeyEscape}
			i++
			continue
		}

		// Ctrl+key (0x01-0x1a except 0x09 tab, 0x0d enter, 0x08/0x7f backspace)
		if data[i] < 0x20 {
			switch data[i] {
			case 0x0d, 0x0a: // CR or LF
				events <- KeyEvent{Key: KeyEnter}
			case 0x09: // Tab
				events <- KeyEvent{Key: KeyTab}
			case 0x08: // Backspace (some terminals)
				events <- KeyEvent{Key: KeyBackspace}
			case 0x03: // Ctrl+C
				events <- KeyEvent{Key: KeyRune, Rune: 'c', Ctrl: true}
			case 0x04: // Ctrl+D
				events <- KeyEvent{Key: KeyRune, Rune: 'd', Ctrl: true}
			case 0x01: // Ctrl+A
				events <- KeyEvent{Key: KeyRune, Rune: 'a', Ctrl: true}
			case 0x05: // Ctrl+E
				events <- KeyEvent{Key: KeyRune, Rune: 'e', Ctrl: true}
			case 0x0b: // Ctrl+K
				events <- KeyEvent{Key: KeyRune, Rune: 'k', Ctrl: true}
			case 0x15: // Ctrl+U
				events <- KeyEvent{Key: KeyRune, Rune: 'u', Ctrl: true}
			case 0x17: // Ctrl+W
				events <- KeyEvent{Key: KeyRune, Rune: 'w', Ctrl: true}
			case 0x0c: // Ctrl+L
				events <- KeyEvent{Key: KeyRune, Rune: 'l', Ctrl: true}
			case 0x1a: // Ctrl+Z
				events <- KeyEvent{Key: KeyRune, Rune: 'z', Ctrl: true}
			default:
				// Generic Ctrl+letter
				events <- KeyEvent{Key: KeyRune, Rune: rune('a' + data[i] - 1), Ctrl: true}
			}
			i++
			continue
		}

		// DEL (backspace on most terminals)
		if data[i] == 0x7f {
			events <- KeyEvent{Key: KeyBackspace}
			i++
			continue
		}

		// UTF-8 multibyte or ASCII printable
		r, size := utf8.DecodeRune(data[i:])
		if r != utf8.RuneError {
			events <- KeyEvent{Key: KeyRune, Rune: r}
			i += size
		} else {
			i++ // skip invalid byte
		}
	}
}

// parseEscape handles escape sequences starting with 0x1b.
// Returns the number of bytes consumed, or 0 if not recognized.
func parseEscape(data []byte, events chan<- Event) int {
	if len(data) < 2 {
		return 0
	}

	// Alt+Backspace: ESC followed by DEL (0x7f)
	if data[1] == 0x7f {
		events <- KeyEvent{Key: KeyBackspace, Alt: true}
		return 2
	}

	// Alt+letter: ESC followed by printable char
	if data[1] >= 0x20 && data[1] < 0x7f && data[1] != '[' && data[1] != 'O' {
		// Alt+Enter is ESC followed by CR
		if data[1] == 0x0d {
			events <- KeyEvent{Key: KeyEnter, Alt: true}
			return 2
		}
		events <- KeyEvent{Key: KeyRune, Rune: rune(data[1]), Alt: true}
		return 2
	}

	// CSI sequences: ESC [
	if data[1] == '[' {
		return parseCSI(data, events)
	}

	// SS3 sequences: ESC O (some terminals send these for function keys)
	if data[1] == 'O' && len(data) >= 3 {
		switch data[2] {
		case 'H':
			events <- KeyEvent{Key: KeyHome}
			return 3
		case 'F':
			events <- KeyEvent{Key: KeyEnd}
			return 3
		}
	}

	return 0
}

// parseCSI handles CSI (ESC [) sequences.
func parseCSI(data []byte, events chan<- Event) int {
	if len(data) < 3 {
		return 0
	}

	// SGR mouse events: ESC [ < button ; x ; y M/m
	if data[2] == '<' {
		return parseSGRMouse(data, events)
	}

	// Bracketed paste start: ESC [ 2 0 0 ~
	if len(data) >= 6 && data[2] == '2' && data[3] == '0' && data[4] == '0' && data[5] == '~' {
		return parseBracketedPaste(data, events)
	}

	// Simple CSI sequences (ESC [ X)
	switch data[2] {
	case 'A':
		events <- KeyEvent{Key: KeyUp}
		return 3
	case 'B':
		events <- KeyEvent{Key: KeyDown}
		return 3
	case 'C':
		events <- KeyEvent{Key: KeyRight}
		return 3
	case 'D':
		events <- KeyEvent{Key: KeyLeft}
		return 3
	case 'H':
		events <- KeyEvent{Key: KeyHome}
		return 3
	case 'F':
		events <- KeyEvent{Key: KeyEnd}
		return 3
	case 'Z': // Shift+Tab
		events <- KeyEvent{Key: KeyTab, Alt: true} // use Alt as shift indicator for tab
		return 3
	}

	// Extended CSI sequences (ESC [ N ~)
	if len(data) >= 4 && data[3] == '~' {
		switch data[2] {
		case '3': // Delete
			events <- KeyEvent{Key: KeyDelete}
			return 4
		case '5': // Page Up
			events <- KeyEvent{Key: KeyPgUp}
			return 4
		case '6': // Page Down
			events <- KeyEvent{Key: KeyPgDown}
			return 4
		}
	}

	// CSI sequences with modifiers: ESC [ 1 ; M X (where M=modifier, X=direction)
	if len(data) >= 6 && data[2] == '1' && data[3] == ';' {
		mod := data[4] - '0'
		ev := KeyEvent{}

		// Modifier bits: 2=Shift, 3=Alt, 4=Shift+Alt, 5=Ctrl, 6=Ctrl+Shift, 7=Ctrl+Alt
		if mod == 3 || mod == 4 || mod == 7 {
			ev.Alt = true
		}
		if mod == 5 || mod == 6 || mod == 7 {
			ev.Ctrl = true
		}

		switch data[5] {
		case 'A':
			ev.Key = KeyUp
		case 'B':
			ev.Key = KeyDown
		case 'C':
			ev.Key = KeyRight
		case 'D':
			ev.Key = KeyLeft
		case 'H':
			ev.Key = KeyHome
		case 'F':
			ev.Key = KeyEnd
		default:
			return 0
		}

		events <- ev
		return 6
	}

	return 0
}

// parseBracketedPaste handles ESC[200~ ... ESC[201~ paste sequences.
// Sends each character as a KeyEvent with no special key handling.
func parseBracketedPaste(data []byte, events chan<- Event) int {
	// Skip the ESC[200~ prefix (6 bytes)
	start := 6
	end := start

	// Find the ESC[201~ terminator
	for end < len(data)-5 {
		if data[end] == 0x1b && end+5 < len(data) &&
			data[end+1] == '[' && data[end+2] == '2' &&
			data[end+3] == '0' && data[end+4] == '1' && data[end+5] == '~' {
			// Found terminator -- parse the pasted content as runes
			content := data[start:end]
			i := 0
			for i < len(content) {
				r, size := utf8.DecodeRune(content[i:])
				if r == '\n' || r == '\r' {
					events <- KeyEvent{Key: KeyEnter, Alt: true} // Alt+Enter = literal newline in paste
				} else if r != utf8.RuneError {
					events <- KeyEvent{Key: KeyRune, Rune: r}
				}
				i += size
			}
			return end + 6 // include terminator
		}
		end++
	}

	// No terminator found in current buffer -- treat as incomplete
	// Return 0 to not consume anything (will be retried with more data)
	return 0
}

// parseSGRMouse handles SGR mouse sequences: ESC [ < button ; x ; y M/m
// SGR button encoding:
//
//	0 = left button, 1 = middle, 2 = right
//	32 = motion flag (added during drag)
//	64 = scroll up, 65 = scroll down
//
// Terminator: M = press/motion, m = release
// Coordinates are 1-based.
func parseSGRMouse(data []byte, events chan<- Event) int {
	// data starts at ESC [ <
	// Find the terminating M or m
	end := 3
	for end < len(data) {
		if data[end] == 'M' || data[end] == 'm' {
			break
		}
		end++
	}
	if end >= len(data) {
		return 0 // incomplete
	}

	release := data[end] == 'm'

	// Parse "button;x;y" between '<' and terminator
	params := data[3:end]
	nums := [3]int{}
	field := 0
	for _, b := range params {
		if b == ';' {
			field++
			if field > 2 {
				break
			}
		} else if b >= '0' && b <= '9' {
			nums[field] = nums[field]*10 + int(b-'0')
		}
	}
	button := nums[0]
	x := nums[1] - 1 // convert to 0-based
	y := nums[2] - 1

	baseButton := button & ^32 // strip motion flag

	// Scroll wheel
	if baseButton == 64 {
		events <- ScrollEvent{Up: true}
		return end + 1
	}
	if baseButton == 65 {
		events <- ScrollEvent{Up: false}
		return end + 1
	}

	// Left button only (baseButton 0)
	if baseButton != 0 {
		return end + 1 // consume but ignore middle/right
	}

	if release {
		events <- MouseEvent{Action: MouseRelease, X: x, Y: y}
	} else if button&32 != 0 {
		events <- MouseEvent{Action: MouseDrag, X: x, Y: y}
	} else {
		events <- MouseEvent{Action: MousePress, X: x, Y: y}
	}
	return end + 1
}
