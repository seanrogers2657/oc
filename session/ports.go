package session

import (
	"context"

	"github.com/srogers/oc/event"
	"github.com/srogers/oc/provider"
	"github.com/srogers/oc/tool"
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
