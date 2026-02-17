// Package tool defines the tool interface and provides built-in tool
// implementations that the AI model can invoke during agentic loops.
//
// The Tool interface requires Name, Description, Parameters (JSON Schema),
// and Execute. Each tool receives a Context with the session ID, working
// directory, cancellation context, and an optional Prompter for asking the
// user questions mid-execution.
//
// Registry is a name→Tool map with insertion-order iteration. It satisfies
// session.ToolExecutor and converts tools to provider.ToolDef for the model.
//
// Built-in tools:
//
//   - bash  — execute shell commands
//   - read  — read file contents
//   - write — create/overwrite files
//   - edit  — find-and-replace in files
//   - glob  — find files by pattern
//   - grep  — search file contents with regex
//   - ask   — ask the user a question (uses Prompter)
//
// path.go contains shared path validation helpers used by file-based tools.
//
// Dependencies: provider (ToolDef type only).
package tool
