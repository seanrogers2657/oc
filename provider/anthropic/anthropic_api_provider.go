package anthropic

import (
	"context"
	"net/http"

	"github.com/srogers/oc/config"
	"github.com/srogers/oc/provider"
)

func NewAnthropicApiProvider(
	config config.Config,
	client *http.Client,
	baseUrl string,
) (provider.Provider, error) {
	if client == nil {
		client = &http.Client{}
	}
	return &anthropicApiProvider{
		auth:    &provider.APIKeyAuth{Key: config.APIKey},
		client:  client,
		baseUrl: baseUrl,
	}, nil
}

type anthropicApiProvider struct {
	auth    provider.Authenticator
	client  *http.Client
	baseUrl string
}

func (p *anthropicApiProvider) Stream(
	ctx context.Context,
	cfg provider.ModelConfig,
	messages []provider.Message,
	tools []provider.ToolDef,
) (<-chan provider.StreamEvent, error) {
	return Stream(ctx, cfg, messages, tools, *p.client, p.auth, p.baseUrl)
}

func (p *anthropicApiProvider) Name() string {
	return "anthropic-api"
}

func (p *anthropicApiProvider) ListModels(ctx context.Context) ([]string, error) {
	return ListModels(ctx, *p.client, p.auth, p.baseUrl)
}
