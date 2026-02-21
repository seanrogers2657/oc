package provider

import (
	"context"
	"net/http"
)

// Provider is the interface every AI backend implements.
type Provider interface {
	// Name returns the provider identifier (e.g., "openai", "anthropic").
	Name() string
	// Stream sends the conversation to the model and returns a channel of events.
	// The channel is closed when the stream ends. Caller must drain it.
	// The ctx can be cancelled to abort the stream.
	Stream(
		ctx context.Context,
		cfg ModelConfig,
		messages []Message,
		tools []ToolDef,
	) (<-chan StreamEvent, error)
}

// Authenticator applies authentication credentials to an HTTP request.
type Authenticator interface {
	Authenticate(req *http.Request) error
}
