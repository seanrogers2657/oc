# oc

A terminal-based AI coding assistant built in Go. Chat with AI models directly from your terminal using a custom TUI with no UI framework dependencies.

## Features

- **Multi-provider support** — works with Anthropic, OpenAI-compatible APIs, and Ollama
- **Agentic tool use** — built-in tools for bash, file read/write/edit, glob, and grep
- **Streaming responses** — real-time token streaming as the model generates
- **Markdown rendering** — styled markdown output directly in the terminal
- **Custom terminal UI** — double-buffered rendering and multi-line input
- **Command palette** — fuzzy-searchable command palette and model picker
- **Configurable keybindings** — customize keyboard shortcuts via JSON config
- **Minimal dependencies** — only `urfave/cli` and `golang.org/x/term`

## Quick Start

```sh
# Anthropic
ANTHROPIC_API_KEY=sk-ant-... go run ./cmd/oc

# Ollama (local)
OC_PROVIDER=ollama OC_MODEL=llama3 go run ./cmd/oc

# Any OpenAI-compatible endpoint
OC_PROVIDER=openai OC_BASE_URL=http://localhost:1234/v1 OC_MODEL=my-model go run ./cmd/oc
```

## Configuration

All configuration is via environment variables:

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
