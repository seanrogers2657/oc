// Package session manages conversation state and orchestrates the agentic
// model loop (stream → tool execute → stream again until the model stops).
//
// A Session holds the message history, token usage, and status (idle/busy).
// The Send method appends a user message and spawns a goroutine running
// runLoop, which repeatedly calls streamOnce (one provider round-trip) and
// executes any tool calls the model requests. Events are published to an
// EventSink so the TUI can react without polling.
//
// Port interfaces (consumer-defined, in ports.go):
//
//   - ModelClient   — streaming AI model (satisfied by provider.*)
//   - ToolExecutor  — tool registry lookup and schema (satisfied by tool.Registry)
//   - EventSink     — event publishing (satisfied by event.Bus)
//
// Key types:
//
//   - Session        — conversation state, mutex-protected
//   - Store          — in-memory session manager (create, get, active)
//   - Message        — rich internal message with Parts (text, reasoning, tool calls)
//   - Part           — interface: TextPart, ReasoningPart, ToolCallPart
//
// Messages with Local=true are displayed in the TUI but excluded from the
// provider message history (used for help text and system notices).
//
// Dependencies: provider (domain types), tool (Context, Result), event (topics).
package session
