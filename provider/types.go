package provider

// Role maps to the standard LLM message roles.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// Message is the provider-agnostic wire format for one conversation turn.
type Message struct {
	Role       Role       // who said this
	Content    string     // text content
	ToolCalls  []ToolCall // tool invocations (assistant only)
	ToolCallID string     // which tool call this responds to (tool role only)
	Name       string     // tool name (tool role only)
}

// ToolCall is a single tool invocation requested by the model.
type ToolCall struct {
	ID   string // unique ID assigned by the model (e.g., "call_abc123")
	Name string // tool name (e.g., "bash")
	Args string // raw JSON arguments
}

// ToolDef describes a tool's schema so the model knows how to call it.
type ToolDef struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON Schema for arguments
}

// StreamEventType discriminates the kind of streaming chunk.
type StreamEventType int

const (
	EventText           StreamEventType = iota // text content delta
	EventToolCallStart                         // new tool call begins (has ID + name)
	EventToolCallDelta                         // incremental args JSON
	EventToolCallEnd                           // tool call arguments fully received
	EventReasoningStart                        // extended thinking begins
	EventReasoningDelta                        // reasoning text delta
	EventReasoningEnd                          // reasoning block ends
	EventDone                                  // stream finished
	EventError                                 // stream error
)

// StreamEvent is one chunk emitted from a streaming response.
type StreamEvent struct {
	Type         StreamEventType
	Text         string    // EventText / EventReasoningDelta
	ToolCall     *ToolCall // EventToolCallStart: ID+Name; EventToolCallDelta: Args chunk
	FinishReason string    // EventDone: "stop" or "tool_calls"
	Usage        *Usage    // EventDone: token consumption
	Error        error     // EventError: what went wrong
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

// ModelConfig holds per-request parameters.
type ModelConfig struct {
	Model       string
	Temperature *float64
	MaxTokens   *int
	TopP        *float64
}
