package tui

// Component is a drawable, updatable UI element.
type Component interface {
	// Update processes an event. Returns true if the component needs redraw.
	Update(ev Event) bool
	// Render draws the component into the buffer within the given bounds.
	Render(buf *ScreenBuffer, bounds Rect)
	// MinSize returns the minimum width and height the component needs.
	MinSize() (width, height int)
}

// FocusableComponent can receive keyboard focus.
type FocusableComponent interface {
	Component
	Focused() bool
	SetFocused(bool)
}
