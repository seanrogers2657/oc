// Package provider defines the AI model communication interface and shared types.
//
// The Provider interface defines a single Stream method that returns a channel
// of StreamEvent values. Implementations live in sub-packages:
//
//   - provider/anthropic — Anthropic Messages API (API key and subscription auth)
//   - provider/openai    — OpenAI-compatible chat completions (also used for
//     Ollama, LM Studio, vLLM, and other compatible servers)
//
// Both implementations parse SSE (Server-Sent Events) streams via shared helpers
// in sse.go and emit a unified StreamEvent type covering text deltas, tool call
// lifecycle (start/delta/end), reasoning (extended thinking), usage, and errors.
//
// Key types:
//
//   - Provider        — port interface (Stream method)
//   - Authenticator   — port interface for HTTP auth (used by BearerAuth in auth/)
//   - Message         — provider-agnostic wire format for conversation turns
//   - ToolDef/ToolCall — tool schema and invocation types
//   - StreamEvent     — discriminated union of streaming chunks
//   - ModelConfig     — per-request params (model, temperature, max_tokens)
//   - Usage           — token consumption counters
//   - RateLimitError  — typed error with RetryAfter for automatic backoff
package provider
