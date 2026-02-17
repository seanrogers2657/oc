// Package auth implements OAuth 2.0 PKCE authentication for Claude Max
// subscriptions.
//
// The flow (in flow.go):
//
//  1. Login generates PKCE params, opens the browser to the authorization URL
//  2. User authorizes and copies the code from the callback page
//  3. Code is exchanged for access + refresh tokens via the token endpoint
//  4. Tokens are persisted to ~/.oc/auth.json by TokenStore
//
// BearerAuth implements provider.Authenticator — it sets the Authorization
// header on API requests and transparently refreshes expired tokens using
// the stored refresh token.
//
// Key types:
//
//   - OAuthConfig  — provider-specific OAuth endpoints and parameters
//   - Token        — access/refresh token pair with expiry
//   - TokenStore   — file-based token persistence (~/.oc/auth.json)
//   - BearerAuth   — implements provider.Authenticator with auto-refresh
//   - PKCEParams   — code verifier, challenge, and state for PKCE flow
//
// Dependencies: stdlib only (net/http, crypto, encoding, os).
package auth
