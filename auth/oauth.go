package auth

// OAuthConfig holds provider-specific OAuth parameters.
// Different providers supply different configs; the auth package
// handles the PKCE flow and token management generically.
type OAuthConfig struct {
	ClientID    string
	AuthURL     string            // authorization endpoint
	TokenURL    string            // token exchange/refresh endpoint
	RedirectURI string            // registered callback URL
	Scope       string            // space-separated scopes
	ExtraParams map[string]string // additional auth URL query params (e.g. "code":"true")
	ExtraHeaders map[string]string // headers to set on authenticated API requests (e.g. "anthropic-beta")
}

// AnthropicOAuth returns the OAuth configuration for Claude Max subscriptions.
func AnthropicOAuth() *OAuthConfig {
	return &OAuthConfig{
		ClientID:    "9d1c250a-e61b-44d9-88ed-5944d1962f5e",
		AuthURL:     "https://claude.ai/oauth/authorize",
		TokenURL:    "https://console.anthropic.com/v1/oauth/token",
		RedirectURI: "https://console.anthropic.com/oauth/code/callback",
		Scope:       "org:create_api_key user:profile user:inference",
		ExtraParams: map[string]string{"code": "true"},
		ExtraHeaders: map[string]string{"anthropic-beta": "oauth-2025-04-20"},
	}
}
