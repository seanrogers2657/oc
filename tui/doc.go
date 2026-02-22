// Package tui is the parent package for the terminal UI subsystem.
// It is organized into two subpackages:
//
//   - tui/common — reusable TUI primitives (stdlib-only): screen buffers,
//     ANSI helpers, input parsing, scrolling, fuzzy matching, and generic
//     widgets (InputArea, CommandPalette, ModelPicker).
//
//   - tui/custom — OC-specific chat UI: main UI orchestrator, message list
//     renderer, status bar, and 3-panel layout computation.
package tui
