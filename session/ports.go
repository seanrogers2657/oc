package session

import (
	"context"

	"github.com/seanrogers2657/oc/event"
	"github.com/seanrogers2657/oc/provider"
	"github.com/seanrogers2657/oc/tool"
)

// ModelClient abstracts AI model interaction.
// Satisfied by: provider.OpenAIProvider, provider.AnthropicProvider
type ModelClient interface {
	Stream(ctx context.Context, cfg provider.ModelConfig, msgs []provider.Message, tools []provider.ToolDef) (<-chan provider.StreamEvent, error)
}

// ToolExecutor abstracts the tool registry.
// Satisfied by: tool.Registry
type ToolExecutor interface {
	Get(name string) (tool.Tool, bool)
	Defs() []provider.ToolDef
}

// EventSink abstracts event publishing.
// Satisfied by: event.Bus
type EventSink interface {
	Publish(topic event.Topic, data interface{})
}
