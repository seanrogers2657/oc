package domain

import "testing"

func TestParseKeyPatternSimple(t *testing.T) {
	tests := []struct {
		input string
		want  KeyEvent
		ok    bool
	}{
		{"enter", KeyEvent{Key: KeyEnter}, true},
		{"tab", KeyEvent{Key: KeyTab}, true},
		{"backspace", KeyEvent{Key: KeyBackspace}, true},
		{"delete", KeyEvent{Key: KeyDelete}, true},
		{"escape", KeyEvent{Key: KeyEscape}, true},
		{"esc", KeyEvent{Key: KeyEscape}, true},
		{"up", KeyEvent{Key: KeyUp}, true},
		{"down", KeyEvent{Key: KeyDown}, true},
		{"left", KeyEvent{Key: KeyLeft}, true},
		{"right", KeyEvent{Key: KeyRight}, true},
		{"home", KeyEvent{Key: KeyHome}, true},
		{"end", KeyEvent{Key: KeyEnd}, true},
		{"pgup", KeyEvent{Key: KeyPgUp}, true},
		{"pgdown", KeyEvent{Key: KeyPgDown}, true},
		{"a", KeyEvent{Key: KeyRune, Rune: 'a'}, true},
		{"/", KeyEvent{Key: KeyRune, Rune: '/'}, true},
		{":", KeyEvent{Key: KeyRune, Rune: ':'}, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := ParseKeyPattern(tt.input)
			if ok != tt.ok {
				t.Fatalf("ParseKeyPattern(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("ParseKeyPattern(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseKeyPatternModifiers(t *testing.T) {
	tests := []struct {
		input string
		want  KeyEvent
	}{
		{"ctrl+c", KeyEvent{Key: KeyRune, Rune: 'c', Mod: ModCtrl}},
		{"ctrl+k", KeyEvent{Key: KeyRune, Rune: 'k', Mod: ModCtrl}},
		{"alt+enter", KeyEvent{Key: KeyEnter, Mod: ModAlt}},
		{"alt+backspace", KeyEvent{Key: KeyBackspace, Mod: ModAlt}},
		{"ctrl+p", KeyEvent{Key: KeyRune, Rune: 'p', Mod: ModCtrl}},
		{"alt+b", KeyEvent{Key: KeyRune, Rune: 'b', Mod: ModAlt}},
		{"alt+f", KeyEvent{Key: KeyRune, Rune: 'f', Mod: ModAlt}},
		{"alt+left", KeyEvent{Key: KeyLeft, Mod: ModAlt}},
		{"alt+right", KeyEvent{Key: KeyRight, Mod: ModAlt}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := ParseKeyPattern(tt.input)
			if !ok {
				t.Fatalf("ParseKeyPattern(%q) returned ok=false", tt.input)
			}
			if got != tt.want {
				t.Errorf("ParseKeyPattern(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseKeyPatternInvalid(t *testing.T) {
	invalid := []string{
		"",
		"ctrl+",
		"abc",
		"ctrl+alt",
	}
	for _, s := range invalid {
		_, ok := ParseKeyPattern(s)
		if ok {
			t.Errorf("ParseKeyPattern(%q) should return ok=false", s)
		}
	}
}

func TestLoadKeyMapBytes(t *testing.T) {
	data := []byte(`{
		"bindings": [
			{"key": "ctrl+c", "action": "exit", "scope": "global"},
			{"key": "enter",  "action": "input.submit", "scope": "input"},
			{"key": "escape", "action": "overlay.close", "scope": "overlay"}
		]
	}`)

	m := NewKeyMap()
	if err := LoadKeyMapBytes(m, data); err != nil {
		t.Fatalf("LoadKeyMapBytes: %v", err)
	}

	// Verify global binding
	action, ok := m.Resolve(KeyScopeGlobal, KeyEvent{Key: KeyRune, Rune: 'c', Mod: ModCtrl})
	if !ok || action != ActionExit {
		t.Errorf("expected ActionExit for Ctrl+c, got %q (ok=%v)", action, ok)
	}

	// Verify input binding
	action, ok = m.Resolve(KeyScopeInput, KeyEvent{Key: KeyEnter})
	if !ok || action != ActionSubmit {
		t.Errorf("expected ActionSubmit for Enter, got %q (ok=%v)", action, ok)
	}

	// Verify overlay binding
	action, ok = m.Resolve(KeyScopeOverlay, KeyEvent{Key: KeyEscape})
	if !ok || action != ActionOverlayClose {
		t.Errorf("expected ActionOverlayClose for Escape, got %q (ok=%v)", action, ok)
	}
}

func TestLoadKeyMapBytesSkipsInvalidKeys(t *testing.T) {
	data := []byte(`{
		"bindings": [
			{"key": "ctrl+c", "action": "exit", "scope": "global"},
			{"key": "invalid_key_name", "action": "nope", "scope": "global"},
			{"key": "enter", "action": "input.submit", "scope": "input"}
		]
	}`)

	m := NewKeyMap()
	if err := LoadKeyMapBytes(m, data); err != nil {
		t.Fatalf("LoadKeyMapBytes: %v", err)
	}

	// Valid bindings should still work
	_, ok := m.Resolve(KeyScopeGlobal, KeyEvent{Key: KeyRune, Rune: 'c', Mod: ModCtrl})
	if !ok {
		t.Error("valid binding should resolve despite invalid entries")
	}
}

func TestLoadKeyMapBytesInvalidJSON(t *testing.T) {
	if err := LoadKeyMapBytes(NewKeyMap(), []byte("not json")); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadKeyMapFileNotExist(t *testing.T) {
	m := NewKeyMap()
	err := LoadKeyMapFile(m, "/nonexistent/path/keybindings.json")
	if err != nil {
		t.Errorf("missing file should return nil, got %v", err)
	}
}
