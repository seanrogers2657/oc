package common

// Event is the interface for all things that can happen in the TUI.
type Event interface {
	EventType() string
}

// ResizeEvent fires when the terminal window changes size.
type ResizeEvent struct {
	Width  int
	Height int
}

func (ResizeEvent) EventType() string { return "resize" }

// CustomEvent carries application-level events from the event bus into the TUI loop.
type CustomEvent struct {
	Topic string
	Data  interface{}
}

func (CustomEvent) EventType() string { return "custom" }

// TickEvent fires on a timer for animations (spinner, etc).
type TickEvent struct{}

func (TickEvent) EventType() string { return "tick" }
