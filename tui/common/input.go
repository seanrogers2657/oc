package common

import (
	"io"
	"unicode/utf8"

	"github.com/srogers/oc/domain"
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
			events <- domain.KeyEvent{Key: domain.KeyEscape}
			i++
			continue
		}

		// Ctrl+key (0x01-0x1a except 0x09 tab, 0x0d enter, 0x08/0x7f backspace)
		if data[i] < 0x20 {
			switch data[i] {
			case 0x0d: // CR (Enter)
				events <- domain.KeyEvent{Key: domain.KeyEnter}
			case 0x0a: // LF (Ctrl+J)
				events <- domain.KeyEvent{Key: domain.KeyRune, Rune: 'j', Mod: domain.ModCtrl}
			case 0x09: // Tab
				events <- domain.KeyEvent{Key: domain.KeyTab}
			case 0x08: // Backspace (some terminals)
				events <- domain.KeyEvent{Key: domain.KeyBackspace}
			case 0x03: // Ctrl+C
				events <- domain.KeyEvent{Key: domain.KeyRune, Rune: 'c', Mod: domain.ModCtrl}
			case 0x04: // Ctrl+D
				events <- domain.KeyEvent{Key: domain.KeyRune, Rune: 'd', Mod: domain.ModCtrl}
			case 0x01: // Ctrl+A
				events <- domain.KeyEvent{Key: domain.KeyRune, Rune: 'a', Mod: domain.ModCtrl}
			case 0x05: // Ctrl+E
				events <- domain.KeyEvent{Key: domain.KeyRune, Rune: 'e', Mod: domain.ModCtrl}
			case 0x0b: // Ctrl+K
				events <- domain.KeyEvent{Key: domain.KeyRune, Rune: 'k', Mod: domain.ModCtrl}
			case 0x15: // Ctrl+U
				events <- domain.KeyEvent{Key: domain.KeyRune, Rune: 'u', Mod: domain.ModCtrl}
			case 0x17: // Ctrl+W
				events <- domain.KeyEvent{Key: domain.KeyRune, Rune: 'w', Mod: domain.ModCtrl}
			case 0x0c: // Ctrl+L
				events <- domain.KeyEvent{Key: domain.KeyRune, Rune: 'l', Mod: domain.ModCtrl}
			case 0x1a: // Ctrl+Z
				events <- domain.KeyEvent{Key: domain.KeyRune, Rune: 'z', Mod: domain.ModCtrl}
			default:
				// Generic Ctrl+letter
				events <- domain.KeyEvent{Key: domain.KeyRune, Rune: rune('a' + data[i] - 1), Mod: domain.ModCtrl}
			}
			i++
			continue
		}

		// DEL (backspace on most terminals)
		if data[i] == 0x7f {
			events <- domain.KeyEvent{Key: domain.KeyBackspace}
			i++
			continue
		}

		// UTF-8 multibyte or ASCII printable
		r, size := utf8.DecodeRune(data[i:])
		if r != utf8.RuneError {
			events <- domain.KeyEvent{Key: domain.KeyRune, Rune: r}
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
		events <- domain.KeyEvent{Key: domain.KeyBackspace, Mod: domain.ModAlt}
		return 2
	}

	// Alt+letter: ESC followed by printable char
	if data[1] >= 0x20 && data[1] < 0x7f && data[1] != '[' && data[1] != 'O' {
		// Alt+Enter is ESC followed by CR
		if data[1] == 0x0d {
			events <- domain.KeyEvent{Key: domain.KeyEnter, Mod: domain.ModAlt}
			return 2
		}
		events <- domain.KeyEvent{Key: domain.KeyRune, Rune: rune(data[1]), Mod: domain.ModAlt}
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
			events <- domain.KeyEvent{Key: domain.KeyHome}
			return 3
		case 'F':
			events <- domain.KeyEvent{Key: domain.KeyEnd}
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

	// Bracketed paste start: ESC [ 2 0 0 ~
	if len(data) >= 6 && data[2] == '2' && data[3] == '0' && data[4] == '0' && data[5] == '~' {
		return parseBracketedPaste(data, events)
	}

	// Simple CSI sequences (ESC [ X)
	switch data[2] {
	case 'A':
		events <- domain.KeyEvent{Key: domain.KeyUp}
		return 3
	case 'B':
		events <- domain.KeyEvent{Key: domain.KeyDown}
		return 3
	case 'C':
		events <- domain.KeyEvent{Key: domain.KeyRight}
		return 3
	case 'D':
		events <- domain.KeyEvent{Key: domain.KeyLeft}
		return 3
	case 'H':
		events <- domain.KeyEvent{Key: domain.KeyHome}
		return 3
	case 'F':
		events <- domain.KeyEvent{Key: domain.KeyEnd}
		return 3
	case 'Z': // Shift+Tab
		events <- domain.KeyEvent{Key: domain.KeyTab, Mod: domain.ModShift}
		return 3
	}

	// Extended CSI sequences (ESC [ N ~)
	if len(data) >= 4 && data[3] == '~' {
		switch data[2] {
		case '3': // Delete
			events <- domain.KeyEvent{Key: domain.KeyDelete}
			return 4
		case '5': // Page Up
			events <- domain.KeyEvent{Key: domain.KeyPgUp}
			return 4
		case '6': // Page Down
			events <- domain.KeyEvent{Key: domain.KeyPgDown}
			return 4
		}
	}

	// CSI sequences with modifiers: ESC [ 1 ; M X (where M=modifier, X=direction)
	if len(data) >= 6 && data[2] == '1' && data[3] == ';' {
		mod := data[4] - '0'
		ev := domain.KeyEvent{}

		// Modifier bits: 2=Shift, 3=Alt, 4=Shift+Alt, 5=Ctrl, 6=Ctrl+Shift, 7=Ctrl+Alt
		if mod == 3 || mod == 4 || mod == 7 {
			ev.Mod |= domain.ModAlt
		}
		if mod == 5 || mod == 6 || mod == 7 {
			ev.Mod |= domain.ModCtrl
		}
		if mod == 2 || mod == 4 || mod == 6 {
			ev.Mod |= domain.ModShift
		}

		switch data[5] {
		case 'A':
			ev.Key = domain.KeyUp
		case 'B':
			ev.Key = domain.KeyDown
		case 'C':
			ev.Key = domain.KeyRight
		case 'D':
			ev.Key = domain.KeyLeft
		case 'H':
			ev.Key = domain.KeyHome
		case 'F':
			ev.Key = domain.KeyEnd
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
					events <- domain.KeyEvent{Key: domain.KeyEnter, Mod: domain.ModAlt} // Alt+Enter = literal newline in paste
				} else if r != utf8.RuneError {
					events <- domain.KeyEvent{Key: domain.KeyRune, Rune: r}
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

