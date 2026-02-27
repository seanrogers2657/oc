package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// bindingsFile is the JSON structure for a keybindings file.
type bindingsFile struct {
	Bindings []bindingEntry `json:"bindings"`
}

type bindingEntry struct {
	Key    string `json:"key"`
	Action string `json:"action"`
	Scope  string `json:"scope"`
}

// LoadKeyMapBytes parses JSON binding data and applies it to the map.
func LoadKeyMapBytes(m *KeyMap, data []byte) error {
	var f bindingsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	for _, entry := range f.Bindings {
		pattern, ok := ParseKeyPattern(entry.Key)
		if !ok {
			continue
		}
		scope := parseScope(entry.Scope)
		m.Bind(scope, pattern, Action(entry.Action))
	}
	return nil
}

// LoadKeyMapFile reads a JSON file and applies bindings to the map.
// Returns nil if the file does not exist.
func LoadKeyMapFile(m *KeyMap, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return LoadKeyMapBytes(m, data)
}

// DefaultBindingsPath returns ~/.oc/keybindings.json.
func DefaultBindingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".oc", "keybindings.json")
}

// ParseKeyPattern converts a string like "ctrl+k", "alt+enter", "a" to a KeyEvent pattern.
func ParseKeyPattern(s string) (KeyEvent, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return KeyEvent{}, false
	}

	var p KeyEvent
	parts := strings.Split(strings.ToLower(s), "+")

	for i, part := range parts {
		part = strings.TrimSpace(part)
		switch part {
		case "ctrl":
			p.Mod |= ModCtrl
		case "alt":
			p.Mod |= ModAlt
		case "shift":
			p.Mod |= ModShift
		default:
			// Must be the last part — the actual key
			if i != len(parts)-1 {
				return KeyEvent{}, false
			}
			switch part {
			case "enter":
				p.Key = KeyEnter
			case "tab":
				p.Key = KeyTab
			case "backspace":
				p.Key = KeyBackspace
			case "delete":
				p.Key = KeyDelete
			case "escape", "esc":
				p.Key = KeyEscape
			case "up":
				p.Key = KeyUp
			case "down":
				p.Key = KeyDown
			case "left":
				p.Key = KeyLeft
			case "right":
				p.Key = KeyRight
			case "home":
				p.Key = KeyHome
			case "end":
				p.Key = KeyEnd
			case "pgup", "pageup":
				p.Key = KeyPgUp
			case "pgdown", "pagedown":
				p.Key = KeyPgDown
			default:
				// Single character
				runes := []rune(part)
				if len(runes) != 1 {
					return KeyEvent{}, false
				}
				r := runes[0]
				// For ctrl combos, normalize to lowercase
				if p.Mod.Has(ModCtrl) {
					r = unicode.ToLower(r)
				}
				p.Key = KeyRune
				p.Rune = r
			}
		}
	}

	// Must have a key set
	if p.Key == KeyNone {
		return KeyEvent{}, false
	}

	return p, true
}

func parseScope(s string) KeyScope {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "overlay":
		return KeyScopeOverlay
	case "input":
		return KeyScopeInput
	default:
		return KeyScopeGlobal
	}
}
