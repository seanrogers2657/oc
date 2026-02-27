package domain

// KeyEvent is a single keystroke from the user.
type KeyEvent struct {
	Key  Key
	Rune rune
	Mod  Mod
}

// EventType identifies this as a key event.
func (KeyEvent) EventType() string { return "key" }

// Matches reports whether e matches the pattern p.
// For non-rune keys the Rune field is ignored.
func (e KeyEvent) Matches(p KeyEvent) bool {
	if e.Key != p.Key || e.Mod != p.Mod {
		return false
	}
	if e.Key == KeyRune && e.Rune != p.Rune {
		return false
	}
	return true
}
