package auth

import (
	"fmt"
	"net/http"
	"sync"
)

// BearerAuth implements provider.Authenticator using OAuth Bearer tokens.
// It automatically refreshes expired tokens using the stored refresh token.
type BearerAuth struct {
	Config *OAuthConfig
	Store  *TokenStore
	Token  *Token
	mu     sync.Mutex
}

// Authenticate sets the Authorization header with a valid Bearer token,
// refreshing transparently if the current token has expired. Any extra
// headers from the OAuthConfig are also applied.
func (b *BearerAuth) Authenticate(req *http.Request) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.Token.Valid() {
		refreshed, err := Refresh(req.Context(), b.Config, b.Token.RefreshToken)
		if err != nil {
			return fmt.Errorf("token refresh failed: %w", err)
		}
		b.Token = refreshed
		b.Store.Save(refreshed)
	}

	req.Header.Set("Authorization", "Bearer "+b.Token.AccessToken)

	for key, value := range b.Config.ExtraHeaders {
		if existing := req.Header.Get(key); existing != "" {
			req.Header.Set(key, existing+","+value)
		} else {
			req.Header.Set(key, value)
		}
	}
	return nil
}
