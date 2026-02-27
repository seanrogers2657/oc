// Package openai implements the provider.Provider interface for
// OpenAI-compatible chat completion APIs.
//
// OpenAIProvider works with OpenAI, Ollama, LM Studio, vLLM, and any
// server that speaks the /v1/chat/completions SSE protocol. Tool support
// is auto-detected from the model's reported capabilities (Ollama) or
// assumed present (OpenAI).
//
// Dependencies: provider (port types).
package openai
