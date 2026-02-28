package custom

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/seanrogers2657/oc/domain"
	"github.com/seanrogers2657/oc/event"
	"github.com/seanrogers2657/oc/tui/common"
)

// Deps holds the injected dependencies for the UI.
type Deps struct {
	Source        EventSource             // subscribe to session events
	OnInput       InputHandler            // called when user submits text
	Status        StatusProvider          // returns current status info
	Messages      MessageListProvider     // returns current messages
	Commands      *common.CommandRegistry // registered commands for command palette
	FetchModels   FetchModels             // fetches available models from provider
	OnModelSelect OnModelSelect           // called when user picks a model
	KeyMap        *domain.KeyMap          // key binding registry
}

// UI is the top-level terminal application.
type UI struct {
	width  int
	height int

	current *common.ScreenBuffer // what's on screen now
	next    *common.ScreenBuffer // what we want on screen

	events chan common.Event
	deps   Deps

	statusBar   *StatusBar
	messageList *MessageList
	inputArea   *common.InputArea
	cmdPalette  *common.CommandPalette
	modelPicker *common.ModelPicker

	layout Layout

	// Scrollback tracking
	prevScrollOffset  int // last known MessageList scroll offset
	scrollbackEmitted int // total lines pushed to terminal scrollback

	// Prompt mode state (for tool asking user a question)
	promptResponse chan<- string // non-nil while awaiting a prompt answer

	// Key bindings
	keyMap *domain.KeyMap

	// FPS counter
	showFPS     bool
	renderCount int
	currentFPS  int
	lastFPSTime time.Time
}

// New creates a new UI with the given dependencies.
func New(deps Deps) *UI {
	t := &UI{
		events: make(chan common.Event, 64),
		deps:   deps,
		keyMap: deps.KeyMap,

		statusBar:   NewStatusBar(deps.Status),
		messageList: NewMessageList(deps.Messages),
		inputArea:   common.NewInputArea(),
	}
	if deps.Commands != nil {
		t.cmdPalette = common.NewCommandPalette(deps.Commands)
	}
	if deps.FetchModels != nil && deps.OnModelSelect != nil {
		t.modelPicker = common.NewModelPicker(deps.FetchModels, deps.OnModelSelect)
	}
	return t
}

// Run starts the UI event loop. Blocks until ctx is cancelled or Ctrl+C.
func (t *UI) Run(ctx context.Context) error {
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

	// Setup terminal — use main screen buffer so content scrolls into scrollback
	w := os.Stdout
	io.WriteString(w, strings.Repeat("\n", t.height)+common.CursorTo(0, 0))
	io.WriteString(w, common.CursorHide+common.BracketedPasteEnable+common.ClearScreen)
	defer func() {
		io.WriteString(w, common.CursorTo(0, t.height-1)+"\n")
		io.WriteString(w, common.BracketedPasteDisable+common.CursorShow+common.ResetStyle)
	}()

	// Initialize buffers
	t.current = common.NewScreenBuffer(t.width, t.height)
	t.next = common.NewScreenBuffer(t.width, t.height)

	// Focus the input area
	t.inputArea.SetFocused(true)

	// Start input reader
	go common.ReadInput(os.Stdin, t.events)

	// Start resize watcher
	go t.watchResize()

	// Start tick timer for animations
	go t.tickLoop(ctx)

	// Subscribe to bus events if source is provided
	if t.deps.Source != nil {
		t.subscribeBusEvents()
	}

	// Wire input submission
	t.inputArea.SetOnSubmit(func(text string) {
		if t.deps.OnInput != nil {
			t.deps.OnInput(text)
		}
	})

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

// handleEvent processes one event. Returns true if the UI should exit.
func (t *UI) handleEvent(ev common.Event, cancel context.CancelFunc) bool {
	switch e := ev.(type) {
	case domain.KeyEvent:
		return t.handleKeyEvent(e, cancel)

	case common.ResizeEvent:
		t.width = e.Width
		t.height = e.Height
		t.current = common.NewScreenBuffer(t.width, t.height)
		t.next = common.NewScreenBuffer(t.width, t.height)
		// Just clear and re-render — no newlines, which would push
		// content (or blank rows) into scrollback on every resize.
		io.WriteString(os.Stdout, common.CursorTo(0, 0)+common.ClearScreen)
		t.computeLayout()
		t.fullRender()

	case common.CustomEvent:
		// Handle prompt events
		if ce, ok := ev.(common.CustomEvent); ok && ce.Topic == string(event.TopicToolPrompt) {
			if req, ok := ce.Data.(event.PromptRequest); ok {
				t.promptResponse = req.Response
				t.inputArea.SetPromptMode(true, req.Question)
				t.inputArea.SetOnSubmit(func(text string) {
					if t.promptResponse != nil {
						t.promptResponse <- text
						t.promptResponse = nil
					}
					t.inputArea.SetPromptMode(false, "")
					t.inputArea.SetOnSubmit(func(text string) {
						if t.deps.OnInput != nil {
							t.deps.OnInput(text)
						}
					})
					t.computeLayout()
				})
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

	case common.TickEvent:
		// Update FPS counter
		if now := time.Now(); now.Sub(t.lastFPSTime) >= time.Second {
			t.currentFPS = t.renderCount
			t.renderCount = 0
			t.lastFPSTime = now
		}

		dirty := t.statusBar.Update(ev)
		dirty = t.messageList.Update(ev) || dirty
		dirty = t.inputArea.Update(ev) || dirty
		if t.showFPS {
			dirty = true
		}
		if dirty {
			t.render()
		}
	}

	return false
}

// handleKeyEvent processes a key event through the KeyMap dispatch chain.
// Returns true if the UI should exit.
func (t *UI) handleKeyEvent(e domain.KeyEvent, cancel context.CancelFunc) bool {
	km := t.keyMap
	if km == nil {
		// No keymap — fall back to legacy Update dispatch
		dirty := t.inputArea.Update(e)
		dirty = t.messageList.Update(e) || dirty
		dirty = t.statusBar.Update(e) || dirty
		if dirty {
			t.computeLayout()
			t.render()
		}
		return false
	}

	// 1. Global exit binding (checked first, always)
	if action, ok := km.Resolve(domain.KeyScopeGlobal, e); ok && action == domain.ActionExit {
		cancel()
		return true
	}

	// 2. Overlay active? Route to overlay scope
	overlayActive := (t.modelPicker != nil && t.modelPicker.Active()) ||
		(t.cmdPalette != nil && t.cmdPalette.Active())

	if overlayActive {
		if action, ok := km.Resolve(domain.KeyScopeOverlay, e); ok {
			dirty, consumed := t.dispatchOverlayAction(action)
			if consumed {
				if dirty {
					t.render()
				}
				return false
			}
		}
		// No overlay action match — try rune insertion for filter
		if e.Key == domain.KeyRune && e.Mod == 0 {
			dirty := t.dispatchOverlayRune(e.Rune)
			if dirty {
				t.render()
			}
			return false
		}
		// Overlay consumes all events
		return false
	}

	// 3. Global actions (scroll, open_palette)
	if action, ok := km.Resolve(domain.KeyScopeGlobal, e); ok {
		switch action {
		case domain.ActionScrollUp:
			t.messageList.ScrollUp(3)
			t.render()
			return false
		case domain.ActionScrollDown:
			t.messageList.ScrollDown(3)
			t.render()
			return false
		case domain.ActionPageUp:
			t.messageList.ScrollUp(t.layout.MessageList.Height / 2)
			t.render()
			return false
		case domain.ActionPageDown:
			t.messageList.ScrollDown(t.layout.MessageList.Height / 2)
			t.render()
			return false
		case domain.ActionOpenPalette:
			// Guard: only open palette when input is empty and not in prompt mode.
			// If the guard fails, fall through to input scope — the ":" or "/"
			// character will be inserted as a normal rune in step 5.
			if t.cmdPalette != nil && !t.inputArea.InPromptMode() {
				if strings.TrimSpace(t.inputArea.Text()) == "" {
					t.cmdPalette.Open()
					t.render()
					return false
				}
			}
		}
	}

	// 4. Input scope actions
	if action, ok := km.Resolve(domain.KeyScopeInput, e); ok {
		if t.inputArea.HandleAction(action) {
			t.computeLayout()
			t.render()
		}
		return false
	}

	// 5. Fallback: printable rune → InputArea
	if e.Key == domain.KeyRune && e.Mod == 0 {
		if t.inputArea.InsertRune(e.Rune) {
			t.computeLayout()
			t.render()
		}
		return false
	}

	return false
}

// dispatchOverlayAction sends an action to the active overlay.
// Returns (dirty, consumed).
func (t *UI) dispatchOverlayAction(action domain.Action) (bool, bool) {
	if t.modelPicker != nil && t.modelPicker.Active() {
		return t.modelPicker.HandleAction(action)
	}
	if t.cmdPalette != nil && t.cmdPalette.Active() {
		return t.cmdPalette.HandleAction(action)
	}
	return false, false
}

// dispatchOverlayRune sends a typed character to the active overlay filter.
func (t *UI) dispatchOverlayRune(r rune) bool {
	if t.modelPicker != nil && t.modelPicker.Active() {
		dirty, _ := t.modelPicker.InsertRune(r)
		return dirty
	}
	if t.cmdPalette != nil && t.cmdPalette.Active() {
		dirty, _ := t.cmdPalette.InsertRune(r)
		return dirty
	}
	return false
}

// computeLayout recalculates panel bounds.
func (t *UI) computeLayout() {
	t.layout = ComputeLayout(t.width, t.height, t.inputArea.LineCount(t.width))
}

// render draws all components to the next buffer and flushes the diff.
func (t *UI) render() {
	// If the terminal has already resized but we haven't processed the
	// ResizeEvent yet, skip this render. Writing rows past the new
	// (shorter) terminal height would cause the terminal to scroll,
	// pushing StatusBar/InputArea into scrollback.
	if w, h, err := term.GetSize(int(os.Stdin.Fd())); err == nil && (w != t.width || h != t.height) {
		return
	}

	t.next.Clear()

	t.statusBar.Render(t.next, t.layout.StatusBar)
	t.messageList.Render(t.next, t.layout.MessageList)
	t.inputArea.Render(t.next, t.layout.InputArea)

	if t.cmdPalette != nil && t.cmdPalette.Active() {
		t.cmdPalette.Render(t.next, t.width, t.height)
	}
	if t.modelPicker != nil && t.modelPicker.Active() {
		t.modelPicker.Render(t.next, t.width, t.height)
	}

	// FPS overlay
	t.renderCount++
	if t.showFPS {
		label := fmt.Sprintf(" %d fps ", t.currentFPS)
		fpsStyle := common.Style{
			FG:   common.NewColor(0, 0, 0),
			BG:   common.NewColor(255, 220, 0),
			Bold: true,
		}
		t.next.WriteString(t.width-len(label), 0, label, fpsStyle)
	}

	// Scrollback: push lines into terminal scrollback when auto-scrolling
	currentOffset, isAutoScroll := t.messageList.ScrollInfo()
	delta := currentOffset - t.prevScrollOffset
	w := os.Stdout

	if delta > 0 && isAutoScroll && t.scrollbackEmitted < 1000 {
		// Cap to MessageList viewport height so we never scroll the
		// StatusBar or InputArea rows into the terminal scrollback.
		if delta > t.layout.MessageList.Height {
			delta = t.layout.MessageList.Height
		}
		// Cap to remaining budget
		if push := 1000 - t.scrollbackEmitted; delta > push {
			delta = push
		}
		// Scroll terminal: newlines at bottom push top rows into scrollback
		io.WriteString(w, common.CursorTo(0, t.height-1)+strings.Repeat("\n", delta))
		t.scrollbackEmitted += delta
		// Full render — viewport content shifted, diff would be wrong
		io.WriteString(w, t.next.FullRender())
	} else {
		diff := t.next.Diff(t.current)
		if diff != "" {
			io.WriteString(w, diff)
		}
	}

	// Only update prevScrollOffset during auto-scroll to avoid
	// spurious deltas when user manually scrolls then resumes
	if isAutoScroll {
		t.prevScrollOffset = currentOffset
	}

	// Park cursor at top-left so terminal resize doesn't push bottom
	// rows (StatusBar, InputArea) into scrollback.
	io.WriteString(os.Stdout, common.CursorTo(0, 0))

	// Swap buffers
	t.current, t.next = t.next, t.current
}

// fullRender draws all components and writes every cell (no diff).
func (t *UI) fullRender() {
	t.next.Clear()

	t.statusBar.Render(t.next, t.layout.StatusBar)
	t.messageList.Render(t.next, t.layout.MessageList)
	t.inputArea.Render(t.next, t.layout.InputArea)

	if t.cmdPalette != nil && t.cmdPalette.Active() {
		t.cmdPalette.Render(t.next, t.width, t.height)
	}
	if t.modelPicker != nil && t.modelPicker.Active() {
		t.modelPicker.Render(t.next, t.width, t.height)
	}

	// FPS overlay
	t.renderCount++
	if t.showFPS {
		label := fmt.Sprintf(" %d fps ", t.currentFPS)
		fpsStyle := common.Style{
			FG:   common.NewColor(0, 0, 0),
			BG:   common.NewColor(255, 220, 0),
			Bold: true,
		}
		t.next.WriteString(t.width-len(label), 0, label, fpsStyle)
	}

	io.WriteString(os.Stdout, t.next.FullRender())

	// Sync scroll offset so render() doesn't see a stale delta
	currentOffset, _ := t.messageList.ScrollInfo()
	t.prevScrollOffset = currentOffset

	// Park cursor at top-left so terminal resize doesn't push bottom
	// rows (StatusBar, InputArea) into scrollback.
	io.WriteString(os.Stdout, common.CursorTo(0, 0))

	// Swap buffers
	t.current, t.next = t.next, t.current
}

// InputArea returns the underlying InputArea component.
func (t *UI) InputArea() *common.InputArea {
	return t.inputArea
}

// OpenModelPicker opens the model picker overlay with the current model highlighted.
func (t *UI) OpenModelPicker() {
	if t.modelPicker != nil {
		status := t.deps.Status()
		t.modelPicker.Open(status.Model)
		t.render()
	}
}

// ToggleFPS toggles the FPS counter overlay.
func (t *UI) ToggleFPS() {
	t.showFPS = !t.showFPS
	if t.showFPS {
		t.renderCount = 0
		t.currentFPS = 0
		t.lastFPSTime = time.Now()
	}
}

// watchResize listens for SIGWINCH and sends ResizeEvents.
func (t *UI) watchResize() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	for range ch {
		fd := int(os.Stdin.Fd())
		w, h, err := term.GetSize(fd)
		if err == nil {
			t.events <- common.ResizeEvent{Width: w, Height: h}
		}
	}
}

// tickLoop sends periodic TickEvents for animations.
func (t *UI) tickLoop(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			select {
			case t.events <- common.TickEvent{}:
			default: // drop tick if channel full
			}
		}
	}
}

// subscribeBusEvents forwards event bus payloads into the UI event channel.
func (t *UI) subscribeBusEvents() {
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
			case t.events <- common.CustomEvent{Topic: string(topic), Data: p.Data}:
			default: // drop if full
			}
		})
	}
}
