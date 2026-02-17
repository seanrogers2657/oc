// Package event provides a thread-safe publish/subscribe event bus for
// decoupling the session layer from the TUI.
//
// The Bus dispatches events by Topic. Subscribers register a Handler for a
// topic and receive a Payload containing the topic and arbitrary data.
// Subscribe returns an unsubscribe function.
//
// Defined topics:
//
//   - TopicPartDelta   — streaming text chunk arrived
//   - TopicPartDone    — a message part finished (e.g. reasoning)
//   - TopicToolStart   — tool execution began
//   - TopicToolDone    — tool execution finished
//   - TopicMsgDone     — full assistant message complete
//   - TopicError       — an error occurred
//   - TopicStatus      — session status changed (idle/busy)
//   - TopicToolPrompt  — tool is asking the user a question (PromptRequest payload)
//
// This package has no dependencies beyond stdlib.
package event
