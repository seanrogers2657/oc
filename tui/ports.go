package tui

import "github.com/srogers/oc/event"

// InputHandler is called when the user submits text from the input area.
type InputHandler func(text string)

// EventSource abstracts subscribing to session events.
// Satisfied by: event.Bus
type EventSource interface {
	Subscribe(topic event.Topic, handler event.Handler) func()
}

// StatusInfo holds data for the status bar display.
// Populated by the composition root from session/provider state.
type StatusInfo struct {
	Model         string // model ID (e.g. "gpt-4o")
	Provider      string // provider name (e.g. "openai")
	Status        string // "idle", "busy"
	Tokens        int    // total tokens used
	ContextTokens int    // input tokens from the most recent request
	Cost          string // formatted cost string (e.g. "$0.03")
	WorkingDir    string // working directory path
}

// StatusProvider returns current status info.
// Satisfied by: a closure from app/ that reads session state.
type StatusProvider func() StatusInfo

// MessageListProvider is defined in messagelist.go -- re-exported here for docs.
// Satisfied by: a closure from app/ that reads session.GetMessages().
