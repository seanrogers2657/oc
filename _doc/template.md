# Plan Template

## File Naming

Name plan files `<nn>-<slug>.md` where `<nn>` is the next two-digit
zero-padded integer in sequence and `<slug>` is a short lowercase hyphenated
description of the work.

```
01-keybindings.md
02-clear-command.md
03-tool-streaming.md
```

The sequence number gives a stable reference ("see plan 03") and a rough
chronology. The slug makes the file recognisable without opening it.
Zero-padding to two digits ensures `ls` sorts correctly past 9.

---

Use this structure for any plan working in this codebase. Fill in the sections
that matter for the task; omit sections that don't apply. Keep it short —
signal over noise.

---

## Status

`draft` | `implemented 2025-01-15`

---

## Goal

One or two sentences. What does "done" look like?

> Example: Add a `/clear` command that resets the current session history
> without restarting the process.

---

## Context

What does the reader need to know before diving in? Link to the right anchors
in `CLAUDE.md` or `ARCHITECTURE.md` rather than re-explaining.

- Which packages are involved?
- Which port interfaces are crossed? (name them explicitly — they are the
  primary architectural boundary in this codebase)
- Any active constraints (e.g. stdlib-only in `domain/`, no new deps)?
- Relevant recent commits or open behaviour that affects the work?

> Example: Touches `session/`, `tui/custom/` (command dispatch), and
> `tui/common/cmdpalette.go`. Crosses the `session.EventSink` port (new topic).
> Must not introduce persistence (see ARCHITECTURE.md §Key Design Decisions #7).

---

## API

The contracts that define or cross the boundary of this work — interfaces,
types, constants, bus topics, anything another package depends on. Focus on
semantic meaning (what it *represents* in the domain) rather than its raw Go
definition. This grounds the approach and makes review easier.

List only the contracts that matter for this plan. Reference the package they
live in so readers know where the canonical definition is. Call out explicitly
anything that is *new* or *changed*, especially if it crosses a package
boundary.

| Name | Kind | Package | Meaning |
|------|------|---------|---------|
| `Message` | type | `session/` | A single turn in the conversation; holds one or more `Part` values (text, tool call, reasoning) |
| `Part` | interface | `session/` | Sealed interface — `TextPart`, `ToolCallPart`, `ReasoningPart` — represents one piece of an assistant response |
| `StreamEvent` | type | `provider/` | A single chunk arriving from the model's SSE stream; typed by `StreamEventType` |
| `Action` | type | `domain/` | A named, scope-aware user intent (e.g. `"submit"`, `"scroll_up"`) resolved from a raw `KeyEvent` |
| `Payload` | type | `event/` | The envelope published on the event bus; carries a `Topic` and an `any` value |
| `TopicSessionCleared` | constant | `event/` | Bus topic published when a session is reset; consumed by `messagelist` |

> Remove rows that don't apply. Add rows for any new names this plan introduces
> or meaningfully changes. Common kinds: `type`, `interface`, `constant`,
> `func`, `method`.

---

## Approach

How will you solve it? Be concrete enough to spot problems before writing code.

- Preferred pattern (new port interface, extend existing type, event on bus, …)
- Data flow change (who produces, who consumes, via what channel/bus topic?)
- Anything that must stay untouched

> Example: Register `clear` in the command palette. On confirm, publish a new
> `TopicSessionCleared` event; `session.Session.Clear()` resets `history`;
> `messagelist` subscribes and flushes its render cache.

### Alternatives Considered

Why is this the right approach? What else was considered and ruled out?

> Example: Considered resetting state directly from the TUI layer, but that
> would couple `tui/custom/` to `session` internals and bypass the port
> boundary. Event-on-bus keeps the layers clean.

---

## Steps

Ordered, completable, independently verifiable chunks of work. Each step
should leave the codebase in a buildable, test-passing state.

- [ ] Step 1 — ...
- [ ] Step 2 — ...
- [ ] Step 3 — ...

---

## Affected Files

List files expected to change so reviewers know what to watch. Mark new files
explicitly — a reviewer should be able to tell at a glance whether a file
should already exist.

| File | Status | Change |
|------|--------|--------|
| `session/session.go` | existing | add `Clear()` method |
| `event/bus.go` | existing | add `TopicSessionCleared` constant |
| `tui/custom/messagelist.go` | existing | subscribe + flush on clear event |
| `tui/common/cmdpalette.go` | existing | register `clear` command |
| `session/clear_test.go` | **new** | unit tests for `Clear()` |

> Status values: `existing`, `**new**`, `~~deleted~~`

---

## Tests

What behaviour needs to be verified, and how?

- [ ] Behaviour: _what the test covers_ — Unit / Integration
- [ ] Behaviour: _what the test covers_ — Unit / Integration

Notes on approach:
- Unit tests: table-driven, mocks defined alongside test file (see `tool/*_test.go`)
- Integration: end-to-end through `session.Send()` → event bus → TUI state
- What can be skipped and why (e.g. pure ANSI rendering is hard to test;
  prefer unit tests on the logic layer instead)

---

## Open Questions

Unresolved decisions or unknowns that could affect the approach. Remove this
section if there are none.

- [ ] Question or unknown — what needs to be decided before / during implementation?

---

## Risks & Watch-outs

Things that could go wrong or decisions that need care.

- Concurrency: session state is mutex-protected; TUI runs on its own goroutine
  — don't hold the lock while publishing to the bus
- Import rules: `domain/` is stdlib-only; `tui/common/` has no app-level deps
- Circular dependency risk: always check the import graph before adding a new
  cross-package import (see `ARCHITECTURE.md §Import Graph`)
- Anything that will break existing keybindings or the 3-scope dispatch order

---

## Definition of Done

Checklist before closing the plan.

- [ ] `go build ./...` passes
- [ ] `go test ./...` passes
- [ ] No new import-graph cycles (`go list -f '{{.ImportPath}} {{.Imports}}' ./...`)
- [ ] Behaviour verified manually in the TUI
- [ ] API section reflects final shape of all new/changed contracts
- [ ] `CLAUDE.md` or `ARCHITECTURE.md` updated if architecture changed
- [ ] Affected `doc.go` package comments updated if package purpose changed
