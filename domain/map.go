package domain

// KeyScope determines when a set of bindings is active.
type KeyScope int

const (
	KeyScopeGlobal  KeyScope = iota // checked first, always active
	KeyScopeOverlay                 // active when a picker/palette is open
	KeyScopeInput                   // active when typing in the input area
	keyScopeCount                   // must be last; used to size arrays
)

// KeyMap is the central registry of key bindings.
// It resolves key events to actions based on scope.
type KeyMap struct {
	bindings [keyScopeCount][]keyBinding // indexed by KeyScope
}

type keyBinding struct {
	pattern KeyEvent
	action  Action
}

// NewKeyMap creates an empty key map with no bindings.
func NewKeyMap() *KeyMap {
	return &KeyMap{}
}

// Bind adds a key pattern -> action mapping in the given scope.
// If the pattern already exists in that scope, it is replaced.
func (m *KeyMap) Bind(scope KeyScope, pattern KeyEvent, action Action) {
	bs := m.bindings[scope]
	for i, b := range bs {
		if b.pattern == pattern {
			bs[i].action = action
			return
		}
	}
	m.bindings[scope] = append(bs, keyBinding{pattern: pattern, action: action})
}

// Resolve looks up the action for a key event in the given scope.
// Returns the action and true, or "" and false if no binding matches.
func (m *KeyMap) Resolve(scope KeyScope, e KeyEvent) (Action, bool) {
	for _, b := range m.bindings[scope] {
		if e.Matches(b.pattern) {
			return b.action, true
		}
	}
	return "", false
}
