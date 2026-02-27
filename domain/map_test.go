package domain

import "testing"

func TestKeyEventMatchesExact(t *testing.T) {
	event := KeyEvent{Key: KeyRune, Rune: 'c', Mod: ModCtrl}
	pattern := KeyEvent{Key: KeyRune, Rune: 'c', Mod: ModCtrl}
	if !event.Matches(pattern) {
		t.Error("expected Ctrl+c event to match Ctrl+c pattern")
	}
}

func TestKeyEventMismatchKey(t *testing.T) {
	event := KeyEvent{Key: KeyEnter}
	pattern := KeyEvent{Key: KeyEscape}
	if event.Matches(pattern) {
		t.Error("Enter event should not match Escape pattern")
	}
}

func TestKeyEventMismatchRune(t *testing.T) {
	event := KeyEvent{Key: KeyRune, Rune: 'a'}
	pattern := KeyEvent{Key: KeyRune, Rune: 'b'}
	if event.Matches(pattern) {
		t.Error("'a' event should not match 'b' pattern")
	}
}

func TestKeyEventMismatchModifiers(t *testing.T) {
	event := KeyEvent{Key: KeyRune, Rune: 'k'}
	pattern := KeyEvent{Key: KeyRune, Rune: 'k', Mod: ModCtrl}
	if event.Matches(pattern) {
		t.Error("plain k should not match Ctrl+k pattern")
	}
}

func TestKeyEventSpecialKeyIgnoresRune(t *testing.T) {
	// For non-KeyRune keys, Rune field should not matter
	event := KeyEvent{Key: KeyEnter, Rune: 'x'} // Rune set but Key is Enter
	pattern := KeyEvent{Key: KeyEnter}
	if !event.Matches(pattern) {
		t.Error("Enter event should match Enter pattern regardless of Rune")
	}
}

func TestKeyMapBindAndResolve(t *testing.T) {
	m := NewKeyMap()
	m.Bind(KeyScopeGlobal, KeyEvent{Key: KeyRune, Rune: 'c', Mod: ModCtrl}, ActionExit)

	action, ok := m.Resolve(KeyScopeGlobal, KeyEvent{Key: KeyRune, Rune: 'c', Mod: ModCtrl})
	if !ok {
		t.Fatal("expected to resolve Ctrl+c")
	}
	if action != ActionExit {
		t.Errorf("expected ActionExit, got %q", action)
	}
}

func TestKeyMapResolveNoMatch(t *testing.T) {
	m := NewKeyMap()
	m.Bind(KeyScopeGlobal, KeyEvent{Key: KeyRune, Rune: 'c', Mod: ModCtrl}, ActionExit)

	_, ok := m.Resolve(KeyScopeGlobal, KeyEvent{Key: KeyRune, Rune: 'k', Mod: ModCtrl})
	if ok {
		t.Error("expected no match for unbound key")
	}
}

func TestKeyMapResolveWrongScope(t *testing.T) {
	m := NewKeyMap()
	m.Bind(KeyScopeInput, KeyEvent{Key: KeyEnter}, ActionSubmit)

	_, ok := m.Resolve(KeyScopeGlobal, KeyEvent{Key: KeyEnter})
	if ok {
		t.Error("input binding should not resolve in global scope")
	}
}

func TestKeyMapBindReplaces(t *testing.T) {
	m := NewKeyMap()
	pattern := KeyEvent{Key: KeyEnter}

	m.Bind(KeyScopeInput, pattern, ActionSubmit)
	m.Bind(KeyScopeInput, pattern, ActionNewline)

	action, ok := m.Resolve(KeyScopeInput, KeyEvent{Key: KeyEnter})
	if !ok {
		t.Fatal("expected to resolve Enter")
	}
	if action != ActionNewline {
		t.Errorf("expected ActionNewline after rebind, got %q", action)
	}

	// Should only have one binding, not two
	if len(m.bindings[KeyScopeInput]) != 1 {
		t.Errorf("expected 1 binding after replacement, got %d", len(m.bindings[KeyScopeInput]))
	}
}

func TestKeyMapMultipleScopes(t *testing.T) {
	m := NewKeyMap()
	m.Bind(KeyScopeGlobal, KeyEvent{Key: KeyEscape}, ActionExit)
	m.Bind(KeyScopeOverlay, KeyEvent{Key: KeyEscape}, ActionOverlayClose)

	action, ok := m.Resolve(KeyScopeGlobal, KeyEvent{Key: KeyEscape})
	if !ok || action != ActionExit {
		t.Errorf("global scope: expected ActionExit, got %q (ok=%v)", action, ok)
	}

	action, ok = m.Resolve(KeyScopeOverlay, KeyEvent{Key: KeyEscape})
	if !ok || action != ActionOverlayClose {
		t.Errorf("overlay scope: expected ActionOverlayClose, got %q (ok=%v)", action, ok)
	}
}
