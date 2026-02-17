// Package tui is a custom terminal rendering engine built from scratch with
// no UI framework dependencies. It drives the full-screen interactive interface.
//
// Architecture: double-buffered cell grid rendering. Two ScreenBuffers (current
// and next) are diffed each frame, emitting only changed ANSI escape sequences.
// The TUI runs a single event loop consuming KeyEvents, ResizeEvents,
// CustomEvents (from the event bus), and TickEvents (for animations).
//
// Layout (top to bottom):
//
//   - MessageList  — scrollable chat history (fills available space)
//   - StatusBar    — one-row model/provider/status/token display
//   - InputArea    — multi-line text editor with history, prompt mode
//
// Overlay:
//
//   - CommandPalette — vim-style ":" command palette with fuzzy matching,
//     rendered on top of the main layout when active
//
// Key components:
//
//   - TUI           — top-level app: event loop, rendering, input dispatch
//   - ScreenBuffer  — 2D cell grid with Set, WriteString, Fill, Diff
//   - InputArea     — multi-line editor with cursor, history, word movement
//   - MessageList   — renders session.Message list with markdown styling
//   - StatusBar     — provider, model, status, token count display
//   - CommandPalette — fuzzy-filtered command list overlay
//   - CommandRegistry/Command — registered actions with names and aliases
//
// Port interfaces (in ports.go):
//
//   - EventSource   — subscribe to session events (satisfied by event.Bus)
//   - InputHandler  — callback for user text submission
//   - StatusProvider — returns current StatusInfo
//   - MessageListProvider — returns current message list
//
// Input (input.go) reads raw terminal bytes and parses them into KeyEvents,
// handling UTF-8, CSI escape sequences, bracketed paste, and modifier keys.
//
// Dependencies: golang.org/x/term (raw mode), session (Message types),
// provider (Role, Usage), event (Topic), markdown (rendering).
package tui
