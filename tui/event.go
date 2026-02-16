package tui

import "github.com/srogers/oc/event"

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
	Topic event.Topic
	Data  interface{}
}

func (CustomEvent) eventType() string { return "custom" }

// ScrollEvent fires when the mouse wheel is scrolled.
type ScrollEvent struct {
	Up bool // true = scroll up, false = scroll down
}

func (ScrollEvent) eventType() string { return "scroll" }

// MouseAction distinguishes press, release, and drag.
type MouseAction int

const (
	MousePress   MouseAction = iota
	MouseRelease
	MouseDrag
)

// MouseEvent fires on mouse button press, release, or drag.
type MouseEvent struct {
	Action MouseAction
	X, Y   int // 0-based screen coordinates
}

func (MouseEvent) eventType() string { return "mouse" }

// TickEvent fires on a timer for animations (spinner, etc).
type TickEvent struct{}

func (TickEvent) eventType() string { return "tick" }
