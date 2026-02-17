package session

import (
	"time"

	"github.com/srogers/oc/provider"
)

// ToolStatus tracks the lifecycle of a tool call.
type ToolStatus int

const (
	ToolPending   ToolStatus = iota // awaiting execution
	ToolRunning                     // currently executing
	ToolCompleted                   // finished successfully
	ToolError                       // finished with error
)

// Part is one segment of a message (text, reasoning, or tool call).
type Part interface {
	partType() string
}

// TextPart holds streamed text content.
type TextPart struct {
	Text string
}

func (TextPart) partType() string { return "text" }

// ReasoningPart holds extended thinking/reasoning content.
type ReasoningPart struct {
	Text string
}

func (ReasoningPart) partType() string { return "reasoning" }

// ToolCallPart holds a tool invocation and its result.
type ToolCallPart struct {
	CallID string
	Tool   string
	Args   string
	Status ToolStatus
	Output string
	Title  string // short summary for TUI display
	Error  string
	Start  time.Time
	End    time.Time
}

func (ToolCallPart) partType() string { return "tool_call" }

// Message is the rich internal representation of a conversation turn.
// Unlike provider.Message (wire format), this preserves streaming structure.
type Message struct {
	ID        string
	SessionID string
	Role      provider.Role
	Parts     []Part
	ModelID   string
	Tokens    provider.Usage
	CreatedAt time.Time
	Error     error
	Local     bool // display-only, not sent to the model
}
