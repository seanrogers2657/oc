package domain

// Key identifies a special key (non-printable).
type Key int

const (
	KeyNone Key = iota
	KeyEnter
	KeyTab
	KeyBackspace
	KeyDelete
	KeyEscape
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyHome
	KeyEnd
	KeyPgUp
	KeyPgDown
	KeyRune // indicates the KeyEvent.Rune field is set
)

// Mod is a bitmask of keyboard modifiers.
type Mod uint8

const (
	ModCtrl  Mod = 1 << iota // Ctrl / Control
	ModAlt                   // Alt / Option
	ModShift                 // Shift
)

// Has reports whether m contains the given modifier flag(s).
func (m Mod) Has(flag Mod) bool { return m&flag != 0 }
