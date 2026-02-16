# `oc` - Architecture

## Overview

Go TUI application for interacting with AI models. Zero dependencies except `urfave/cli` (CLI framework) and `golang.org/x/term` (terminal raw mode). Everything else built from scratch using Go stdlib.

**Constraints:**
- In-memory storage only (no persistence for v1)
- Providers: OpenAI-compatible + Anthropic (covers OpenAI, Ollama, LM Studio, vLLM, Claude)
- Custom TUI engine using raw ANSI escape codes

---

## Hexagonal Architecture

Each package follows ports & adapters: domain types and logic live alongside port interfaces
(defined at the consumer). Concrete adapters implement ports. `cmd/oc/` is the composition root
that wires adapters to ports. Go's implicit interface satisfaction means no explicit "implements".

### Port-Adapter Wiring

| Port (consumer-defined) | Satisfied By |
|--------------------------|-------------|
| `session.ModelClient` | `provider.OpenAIProvider`, `provider.AnthropicProvider` |
| `session.ToolExecutor` | `tool.Registry` |
| `session.EventSink` | `event.Bus` |
| `tui.SessionView` | `session.Session` |
| `tui.EventSource` | `event.Bus` |
| `tui.InputHandler` | closure from `cmd/oc/` calling `session.Send()` |

---

## Package Structure

```
oc/
  go.mod
  go.sum
  cmd/
    oc/
      main.go                          # Entry point + composition root: urfave/cli, wires adapters to ports
  config/
    config.go                          # Env vars, provider config, model defaults
  provider/
    types.go                           # Domain: Role, Message, ToolCall, ToolDef, StreamEvent, Usage, ModelConfig
    provider.go                        # Port: Provider interface
    openai.go                          # Adapter: OpenAI-compatible API (streaming via SSE)
    anthropic.go                       # Adapter: Anthropic API (streaming via SSE)
    sse.go                             # Infrastructure: shared SSE stream parser for adapters
    message.go                         # Domain: format conversion helpers
  tool/
    tool.go                            # Port: Tool interface + domain types (Result, Context)
    registry.go                        # Domain: Registry (lookup, conversion to ToolDef)
    bash.go                            # Adapter: execute shell commands (os/exec)
    read.go                            # Adapter: read files with line numbers
    write.go                           # Adapter: write/create files
    edit.go                            # Adapter: string-replacement editing
    glob.go                            # Adapter: filepath.WalkDir + pattern matching
    grep.go                            # Adapter: regex search across files
  session/
    ports.go                           # Outbound ports: ModelClient, ToolExecutor, EventSink
    message.go                         # Domain: Message, Part types (TextPart, ToolCallPart, ReasoningPart)
    session.go                         # Domain: Session, Store, SessionStatus
    loop.go                            # Domain logic: stream -> tool detect -> execute -> repeat
  event/
    bus.go                             # Infrastructure: Topic, Payload, Handler, Bus
  tui/
    ports.go                           # Inbound ports: SessionView, EventSource, InputHandler
    tui.go                             # Domain: lifecycle, alt screen, raw mode, main event loop
    screen.go                          # Domain: ScreenBuffer, Cell, Style, Color, double-buffering
    ansi.go                            # Adapter: ANSI escape code generation (screen buffer -> terminal)
    input.go                           # Adapter: raw stdin reader (terminal -> KeyEvent)
    event.go                           # Domain: Event types (KeyEvent, ResizeEvent, CustomEvent)
    layout.go                          # Domain: Rect computation, fixed 3-panel layout
    component.go                       # Domain: Component + FocusableComponent interfaces
    scroll.go                          # Domain: ScrollState (offset, viewport, content height)
    messagelist.go                     # Domain: scrollable chat history (reads SessionView port)
    inputarea.go                       # Domain: multi-line text editor (calls InputHandler port)
    statusbar.go                       # Domain: model info, token count (reads SessionView port)
    tooloutput.go                      # Domain: tool execution result display
    markdown.go                        # Domain: maps markdown.SpanKind -> tui.Style
  markdown/
    render.go                          # Pure domain: Markdown string -> []Line (no dependencies)
```

---

## Import Graph

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

No circular dependencies. `cmd/oc/` is the only package that knows about all concrete types.

---

## Data Flow

### Main Message Loop

```
User types in InputArea
  -> session.Loop() creates user Message, appends to history
  -> Converts history to provider.Messages
  -> provider.Stream(ctx, config, messages, toolDefs) returns <-chan StreamEvent
  -> Read events from channel:
      EventText          -> append to TextPart, publish PartDelta to bus
      EventToolCallStart -> create ToolCallPart (pending)
      EventToolCallDelta -> accumulate args JSON
      EventToolCallEnd   -> execute tool, set result, append ToolResult to history
      EventReasoningX    -> accumulate ReasoningPart
      EventDone          -> if finish_reason=="tool_calls", loop again
                            if finish_reason=="stop", break
      EventError         -> set error, break
```

### Streaming Updates to TUI

```
provider goroutine  ->  session.Loop goroutine  ->  event.Bus  ->  TUI event loop
                        (processes events,          (dispatches)    (injects CustomEvent,
                         updates state)                              calls Component.Update,
                                                                     re-renders dirty regions)
```

### Tool Execution

```
EventToolCallEnd received
  -> toolRegistry.Get(name)
  -> tool.Execute(ctx, argsJSON)
  -> Append tool result as Message{Role: RoleTool}
  -> Loop continues (provider called again with updated history)
```

---

## TUI Rendering Engine

### Double-Buffered Screen

Maintain `current` (on-screen) and `next` (desired) buffers. On flush, diff cells and emit only ANSI for changes. Flicker-free.

### Fixed 3-Panel Layout

```
+----------------------------------+
| StatusBar (1 row)                |  Model | Provider | Tokens | Cost
+----------------------------------+
|                                  |
| MessageList (fills middle)       |  Scrollable chat history
|                                  |
+----------------------------------+
| InputArea (3-N rows)             |  > multi-line editor
| hint: Enter=send Ctrl+C=cancel  |
+----------------------------------+
```

Layout computation:
```
statusBarH = 1
inputAreaH = max(3, min(inputLines + 2, totalH / 3))
messageListH = totalH - statusBarH - inputAreaH
```

### Input Handling

Raw stdin reader goroutine using `golang.org/x/term.MakeRaw()`. State machine parses single ASCII bytes, multi-byte UTF-8, escape sequences (arrows, home/end, pgup/pgdn, delete), and bracketed paste.

### Resize Handling

SIGWINCH listener goroutine pushes `ResizeEvent` into the event channel.

---

## Key Design Decisions

1. **Hexagonal architecture** -- Consumer-defined ports, Deps injection, `cmd/oc/` as sole composition root
2. **Channel-based streaming** -- Provider returns `<-chan StreamEvent`, idiomatic Go, works with `select`
3. **Event bus for decoupling** -- Session publishes to bus, TUI subscribes. Neither knows about the other
4. **Double-buffered screen** -- Diff current vs desired, emit only changed cells
5. **Fixed 3-panel layout** -- No flexbox engine needed
6. **Synchronous tool execution in loop** -- Block on tool before continuing stream. TUI stays responsive on separate goroutine
7. **In-memory only** -- All state in Go structs. Persistence can be added later behind interfaces
