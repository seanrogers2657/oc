# oc - Terminal AI Coding Assistant

A Go TUI application for interacting with AI models (Anthropic, OpenAI-compatible). Custom terminal rendering engine built from scratch with zero UI framework dependencies.

## Quick Start

```
ANTHROPIC_API_KEY=sk-ant-... go run ./cmd/oc
OC_PROVIDER=openai OC_BASE_URL=http://localhost:11434/v1 OC_MODEL=llama3 go run ./cmd/oc
```

## Architecture

Hexagonal architecture with consumer-defined port interfaces. `cmd/oc/` is the composition root that wires adapters to ports. Go's implicit interface satisfaction means no explicit "implements".

### Package Dependency Graph

```
cmd/oc/   -> config/, provider/, tool/, session/, event/, tui/  (composition root)
tui/      -> session/ (Message types), provider/ (Role, Usage), event/ (Topic), markdown/
session/  -> provider/ (domain types), tool/ (Context, Result), event/ (Topic)
tool/     -> provider/ (ToolDef)
markdown/ -> (stdlib only)
config/   -> (stdlib only)
event/    -> (stdlib only)
provider/ -> (stdlib only)
```

No circular dependencies. All behavior flows through port interfaces.

### Key Packages

| Package | Purpose | Key Files |
|---------|---------|-----------|
| `cmd/oc/` | Entry point, CLI flags, wires all packages | `main.go` |
| `provider/` | AI model communication (streaming) | `provider.go` (port), `openai.go`, `anthropic.go`, `sse.go` |
| `session/` | Conversation state, agentic loop | `ports.go`, `session.go`, `loop.go`, `message.go` |
| `tool/` | Tool interface and implementations | `tool.go` (port), `registry.go`, `bash.go`, `read.go`, `write.go`, `edit.go`, `glob.go`, `grep.go` |
| `tui/` | Terminal rendering engine | `tui.go`, `screen.go`, `ansi.go`, `input.go`, `messagelist.go`, `inputarea.go`, `statusbar.go` |
| `event/` | Decoupled pub/sub event bus | `bus.go` |
| `markdown/` | Pure markdown-to-styled-text parser | `render.go` |
| `config/` | Env var configuration | `config.go` |

### Data Flow

```
User types in InputArea
  -> session.Send() creates user Message, spawns goroutine
  -> runLoop() calls streamOnce() repeatedly
  -> provider.Stream() returns <-chan StreamEvent
  -> Events accumulate into assistant Message parts (text, tool calls, reasoning)
  -> Tool calls executed, results appended to history
  -> Loop continues until model says "stop"
  -> event.Bus notifies TUI of changes
  -> TUI re-renders affected components
```

### Port Interfaces

- `session.ModelClient` <- satisfied by `provider.{OpenAI,Anthropic}Provider`
- `session.ToolExecutor` <- satisfied by `tool.Registry`
- `session.EventSink` <- satisfied by `event.Bus`
- `tui.EventSource` <- satisfied by `event.Bus`
- `tui.InputHandler` <- closure from `cmd/oc/`

### TUI Layout

```
MessageList  (fills top, scrollable)
StatusBar    (1 row, dark background separator)
InputArea    (bottom, multi-line editor)
```

Double-buffered rendering: diff current vs desired screen buffer, emit only changed ANSI cells. Mouse tracking enabled (SGR mode) for scroll and text selection.

## Conventions

- Only external deps: `urfave/cli/v2` (CLI framework), `golang.org/x/term` (raw mode)
- In-memory only, no persistence
- Errors from the model appear as assistant messages in the chat, not in the status bar
- User messages render with `> ` prefix in light gray; assistant messages indent 2 spaces
- All streaming uses channel-based `<-chan StreamEvent` pattern
- Session state is mutex-protected; TUI runs on its own goroutine
- Tests use table-driven style where appropriate; mocks are defined alongside tests

## Commands

```
go build ./...          # build
go test ./...           # run all tests (213 tests across 8 packages)
go run ./cmd/oc         # run the application
```

## Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `OC_PROVIDER` | Provider name | `anthropic`, `openai`, `ollama` |
| `OC_MODEL` | Model ID | `claude-sonnet-4-20250514`, `gpt-4o`, `llama3` |
| `OC_API_KEY` | API key (overrides provider-specific) | `sk-...` |
| `OC_BASE_URL` | Base URL for OpenAI-compatible endpoints | `http://localhost:11434/v1` |
| `ANTHROPIC_API_KEY` | Anthropic API key (fallback) | `sk-ant-...` |
| `OPENAI_API_KEY` | OpenAI API key (fallback) | `sk-...` |
| `OC_TEMPERATURE` | Sampling temperature | `0.7` |
| `OC_MAX_TOKENS` | Max output tokens | `4096` |
