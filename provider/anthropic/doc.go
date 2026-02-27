// Package anthropic implements the provider.Provider interface for the
// Anthropic Messages API.
//
// Two authentication modes are supported:
//
//   - API key auth (AnthropicAPIProvider) — uses a static API key header
//   - Subscription auth (AnthropicSubscriptionProvider) — uses the auth.Authenticator
//     interface for Claude Max OAuth token management
//
// Both providers share the same SSE streaming parser and message format
// conversion logic defined in anthropic_base.go.
//
// Dependencies: provider (port types).
package anthropic
