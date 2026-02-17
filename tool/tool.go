package tool

import "context"

// Result is what a tool returns after execution.
type Result struct {
	Output string // text output shown to the model
	Title  string // short summary for TUI display
	Error  error  // nil on success
}

// Prompter allows a tool to ask the user a question and wait for a response.
type Prompter interface {
	Prompt(ctx context.Context, question string) (string, error)
}

// Context carries per-invocation state into a tool.
type Context struct {
	SessionID  string
	WorkingDir string
	Ctx        context.Context
	Prompter   Prompter // nil if prompting not available
}

// Tool is the interface all tools implement.
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Execute(ctx Context, argsJSON string) Result
}
