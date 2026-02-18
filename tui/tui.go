package tui

import (
	"context"
	"io"
	"os"
	"os/signal"
	"strings"
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
	Commands *CommandRegistry    // registered commands for command palette
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
	cmdPalette  *CommandPalette

	layout Layout

	// Prompt mode state (for tool asking user a question)
	promptResponse chan<- string // non-nil while awaiting a prompt answer
}

// New creates a new TUI with the given dependencies.
func New(deps Deps) *TUI {
	t := &TUI{
		events: make(chan Event, 64),
		deps:   deps,

		statusBar:   NewStatusBar(deps.Status),
		messageList: NewMessageList(deps.Messages),
		inputArea:   NewInputArea(),
	}
	if deps.Commands != nil {
		t.cmdPalette = NewCommandPalette(deps.Commands)
	}
	return t
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
	io.WriteString(w, AltScreenEnter+CursorHide+BracketedPasteEnable+ClearScreen)
	defer io.WriteString(w, BracketedPasteDisable+CursorShow+AltScreenExit)

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

		// Command palette intercept - when active, it gets ALL keys
		if t.cmdPalette != nil && t.cmdPalette.Active() {
			dirty, consumed := t.cmdPalette.Update(e)
			if consumed {
				if dirty {
					t.render()
				}
				return false
			}
		}

		// Trigger command palette: ':' when input is empty and not in prompt mode
		if t.cmdPalette != nil && e.Key == KeyRune && e.Rune == ':' && !t.inputArea.promptMode {
			if strings.TrimSpace(t.inputArea.Text()) == "" {
				t.cmdPalette.Open()
				t.render()
				return false
			}
		}

		// Scroll message list with Ctrl+K (up) / Ctrl+J (down)
		if e.Ctrl && e.Rune == 'k' {
			t.messageList.ScrollUp(3)
			t.render()
			return false
		}
		if e.Ctrl && e.Rune == 'j' {
			t.messageList.ScrollDown(3)
			t.render()
			return false
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

	case CustomEvent:
		// Handle prompt events
		if ce, ok := ev.(CustomEvent); ok && ce.Topic == event.TopicToolPrompt {
			if req, ok := ce.Data.(event.PromptRequest); ok {
				t.promptResponse = req.Response
				t.inputArea.SetPromptMode(true, req.Question)
				t.inputArea.onSubmit = func(text string) {
					if t.promptResponse != nil {
						t.promptResponse <- text
						t.promptResponse = nil
					}
					t.inputArea.SetPromptMode(false, "")
					t.inputArea.onSubmit = func(text string) {
						if t.deps.OnInput != nil {
							t.deps.OnInput(text)
						}
					}
					t.computeLayout()
				}
				t.computeLayout()
				t.render()
				break
			}
		}
		dirty := t.statusBar.Update(ev)
		dirty = t.messageList.Update(ev) || dirty
		dirty = t.inputArea.Update(ev) || dirty
		if dirty {
			t.render()
		}

	case TickEvent:
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
	t.layout = ComputeLayout(t.width, t.height, t.inputArea.LineCount(t.width))
}

// render draws all components to the next buffer and flushes the diff.
func (t *TUI) render() {
	t.next.Clear()

	t.statusBar.Render(t.next, t.layout.StatusBar)
	t.messageList.Render(t.next, t.layout.MessageList)
	t.inputArea.Render(t.next, t.layout.InputArea)

	if t.cmdPalette != nil && t.cmdPalette.Active() {
		t.cmdPalette.Render(t.next, t.width, t.height)
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

	if t.cmdPalette != nil && t.cmdPalette.Active() {
		t.cmdPalette.Render(t.next, t.width, t.height)
	}

	io.WriteString(os.Stdout, t.next.FullRender())

	// Swap buffers
	t.current, t.next = t.next, t.current
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
		event.TopicToolPrompt,
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
