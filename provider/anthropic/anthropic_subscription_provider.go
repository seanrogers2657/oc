package anthropic

import (
	"context"
	"fmt"
	"net/http"

	"github.com/srogers/oc/auth"
	"github.com/srogers/oc/config"
	"github.com/srogers/oc/provider"
)

func NewAnthropicSubscriptionProvider(
	config config.Config,
	client *http.Client,
	baseUrl string,
) (provider.Provider, error) {
	// Try stored OAuth token (from 'oc login')
	oauthCfg := auth.AnthropicOAuth()
	store := auth.NewTokenStore()
	token, err := store.Load()
	if client == nil {
		client = &http.Client{}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load auth token: %w", err)
	}
	if token != nil {
		auth := &auth.BearerAuth{Config: oauthCfg, Store: store, Token: token}
		return &anthropicSubscriptionProvider{
			auth:    auth,
			client:  client,
			baseUrl: baseUrl,
		}, nil
	} else if config.Provider == "claude-max" {
		return nil, fmt.Errorf("not authenticated — run 'oc login' first")
	} else {
		return nil, fmt.Errorf("API key required for Anthropic (set ANTHROPIC_API_KEY or OC_API_KEY, or run 'oc login')")
	}
}

type anthropicSubscriptionProvider struct {
	auth    provider.Authenticator
	client  *http.Client
	baseUrl string
}

func (p *anthropicSubscriptionProvider) Stream(
	ctx context.Context,
	cfg provider.ModelConfig,
	messages []provider.Message,
	tools []provider.ToolDef,
) (<-chan provider.StreamEvent, error) {
	return Stream(ctx, cfg, messages, tools, *p.client, p.auth, p.baseUrl)
}

func (p *anthropicSubscriptionProvider) Name() string {
	return "claude-max"
}
