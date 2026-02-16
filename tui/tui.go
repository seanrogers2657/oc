package tui

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/srogers/oc/event"
)

// Deps holds the injected dependencies for the TUI.
type Deps struct {
	Source   EventSource         // subscribe to session events
	OnInput  InputHandler        // called when user submits text
	Status   StatusProvider      // returns current status info
	Messages MessageListProvider // returns current messages
}

// TUI is the top-level terminal application.
type TUI struct {
	width  int
	height int

	current *ScreenBuffer // what's on screen now
	next    *ScreenBuffer // what we want on screen

	events chan Event
	deps   Deps

	statusBar   *StatusBar
	messageList *MessageList
	inputArea   *InputArea

	layout Layout

	// Selection state
	selecting bool // true while mouse button is held
	selAnchor pos  // where the press started
	selCursor pos  // current drag position
	hasSelect bool // true if there's a visible selection
}

// pos is a screen coordinate.
type pos struct{ X, Y int }

// New creates a new TUI with the given dependencies.
func New(deps Deps) *TUI {
	return &TUI{
		events: make(chan Event, 64),
		deps:   deps,

		statusBar:   NewStatusBar(deps.Status),
		messageList: NewMessageList(deps.Messages),
		inputArea:   NewInputArea(),
	}
}

// Run starts the TUI event loop. Blocks until ctx is cancelled or Ctrl+C.
func (t *TUI) Run(ctx context.Context) error {
	// Get initial terminal size
	fd := int(os.Stdin.Fd())
	width, height, err := term.GetSize(fd)
	if err != nil {
		// Fallback
		width, height = 80, 24
	}
	t.width = width
	t.height = height

	// Enter raw mode
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer term.Restore(fd, oldState)

	// Setup terminal
	w := os.Stdout
	io.WriteString(w, AltScreenEnter+CursorHide+BracketedPasteEnable+MouseEnable+ClearScreen)
	defer io.WriteString(w, MouseDisable+BracketedPasteDisable+CursorShow+AltScreenExit)

	// Initialize buffers
	t.current = NewScreenBuffer(t.width, t.height)
	t.next = NewScreenBuffer(t.width, t.height)

	// Focus the input area
	t.inputArea.SetFocused(true)

	// Start input reader
	go ReadInput(os.Stdin, t.events)

	// Start resize watcher
	go t.watchResize()

	// Start tick timer for animations
	go t.tickLoop(ctx)

	// Subscribe to bus events if source is provided
	if t.deps.Source != nil {
		t.subscribeBusEvents()
	}

	// Wire input submission
	t.inputArea.onSubmit = func(text string) {
		if t.deps.OnInput != nil {
			t.deps.OnInput(text)
		}
	}

	// Initial render
	t.computeLayout()
	t.render()

	// Create a cancellable context for clean shutdown
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Main event loop
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-t.events:
			if !ok {
				return nil
			}
			if t.handleEvent(ev, cancel) {
				return nil
			}
		}
	}
}

// handleEvent processes one event. Returns true if the TUI should exit.
func (t *TUI) handleEvent(ev Event, cancel context.CancelFunc) bool {
	switch e := ev.(type) {
	case KeyEvent:
		// Global keybindings
		if e.Ctrl && e.Rune == 'c' {
			cancel()
			return true
		}

		// Clear selection on any keypress
		if t.hasSelect {
			t.hasSelect = false
		}

		// Forward to all components
		dirty := t.inputArea.Update(ev)
		dirty = t.messageList.Update(ev) || dirty
		dirty = t.statusBar.Update(ev) || dirty

		if dirty {
			t.computeLayout()
			t.render()
		}

	case ResizeEvent:
		t.width = e.Width
		t.height = e.Height
		t.current = NewScreenBuffer(t.width, t.height)
		t.next = NewScreenBuffer(t.width, t.height)
		// Clear terminal so stale content from old size is wiped
		io.WriteString(os.Stdout, ClearScreen)
		t.computeLayout()
		t.fullRender()

	case MouseEvent:
		switch e.Action {
		case MousePress:
			t.selecting = true
			t.selAnchor = pos{e.X, e.Y}
			t.selCursor = t.selAnchor
			t.hasSelect = false
			// Clear any existing selection highlight
			t.render()
		case MouseDrag:
			if t.selecting {
				t.selCursor = pos{e.X, e.Y}
				t.hasSelect = t.selAnchor != t.selCursor
				t.render()
			}
		case MouseRelease:
			if t.selecting {
				t.selCursor = pos{e.X, e.Y}
				t.selecting = false
				t.hasSelect = t.selAnchor != t.selCursor
				t.render()
			}
		}

	case ScrollEvent:
		if t.messageList.Update(ev) {
			t.render()
		}

	case CustomEvent, TickEvent:
		dirty := t.statusBar.Update(ev)
		dirty = t.messageList.Update(ev) || dirty
		dirty = t.inputArea.Update(ev) || dirty
		if dirty {
			t.render()
		}
	}

	return false
}

// computeLayout recalculates panel bounds.
func (t *TUI) computeLayout() {
	t.layout = ComputeLayout(t.width, t.height, t.inputArea.LineCount())
}

// render draws all components to the next buffer and flushes the diff.
func (t *TUI) render() {
	t.next.Clear()

	t.statusBar.Render(t.next, t.layout.StatusBar)
	t.messageList.Render(t.next, t.layout.MessageList)
	t.inputArea.Render(t.next, t.layout.InputArea)

	if t.hasSelect || t.selecting {
		t.applySelection(t.next)
	}

	// Compute diff and write to stdout
	diff := t.next.Diff(t.current)
	if diff != "" {
		io.WriteString(os.Stdout, diff)
	}

	// Swap buffers
	t.current, t.next = t.next, t.current
}

// fullRender draws all components and writes every cell (no diff).
// Used after resize or screen clear when the terminal state is unknown.
func (t *TUI) fullRender() {
	t.next.Clear()

	t.statusBar.Render(t.next, t.layout.StatusBar)
	t.messageList.Render(t.next, t.layout.MessageList)
	t.inputArea.Render(t.next, t.layout.InputArea)

	if t.hasSelect || t.selecting {
		t.applySelection(t.next)
	}

	io.WriteString(os.Stdout, t.next.FullRender())

	// Swap buffers
	t.current, t.next = t.next, t.current
}

// selectionRange returns the start and end positions in reading order.
func (t *TUI) selectionRange() (start, end pos) {
	a, b := t.selAnchor, t.selCursor
	if b.Y < a.Y || (b.Y == a.Y && b.X < a.X) {
		a, b = b, a
	}
	return a, b
}

// applySelection overlays a highlight style on selected cells.
func (t *TUI) applySelection(buf *ScreenBuffer) {
	start, end := t.selectionRange()
	for y := start.Y; y <= end.Y && y < buf.Height; y++ {
		if y < 0 {
			continue
		}
		sx := 0
		ex := buf.Width - 1
		if y == start.Y {
			sx = start.X
		}
		if y == end.Y {
			ex = end.X
		}
		for x := sx; x <= ex && x < buf.Width; x++ {
			if x < 0 {
				continue
			}
			cell := &buf.Cells[y][x]
			cell.Style.BG = NewColor(60, 60, 100)
			if !cell.Style.FG.Set {
				cell.Style.FG = NewColor(220, 220, 220)
			}
		}
	}
}


// watchResize listens for SIGWINCH and sends ResizeEvents.
func (t *TUI) watchResize() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	for range ch {
		fd := int(os.Stdin.Fd())
		w, h, err := term.GetSize(fd)
		if err == nil {
			t.events <- ResizeEvent{Width: w, Height: h}
		}
	}
}

// tickLoop sends periodic TickEvents for animations.
func (t *TUI) tickLoop(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			select {
			case t.events <- TickEvent{}:
			default: // drop tick if channel full
			}
		}
	}
}

// subscribeBusEvents forwards event bus payloads into the TUI event channel.
func (t *TUI) subscribeBusEvents() {
	topics := []event.Topic{
		event.TopicPartDelta,
		event.TopicPartDone,
		event.TopicToolStart,
		event.TopicToolDone,
		event.TopicMsgDone,
		event.TopicError,
		event.TopicStatus,
	}
	for _, topic := range topics {
		topic := topic
		t.deps.Source.Subscribe(topic, func(p event.Payload) {
			select {
			case t.events <- CustomEvent{Topic: topic, Data: p.Data}:
			default: // drop if full
			}
		})
	}
}
