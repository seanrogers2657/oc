package common

// Event is the interface for all things that can happen in the TUI.
type Event interface {
	eventType() string
}

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
	KeyRune // indicates Rune field is set
)

// KeyEvent is a single keystroke from the user.
type KeyEvent struct {
	Key  Key
	Rune rune
	Ctrl bool
	Alt  bool
}

func (KeyEvent) eventType() string { return "key" }

// ResizeEvent fires when the terminal window changes size.
type ResizeEvent struct {
	Width  int
	Height int
}

func (ResizeEvent) eventType() string { return "resize" }

// CustomEvent carries application-level events from the event bus into the TUI loop.
type CustomEvent struct {
	Topic string
	Data  interface{}
}

func (CustomEvent) eventType() string { return "custom" }

// TickEvent fires on a timer for animations (spinner, etc).
type TickEvent struct{}

func (TickEvent) eventType() string { return "tick" }
