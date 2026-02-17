package auth

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Login performs a full OAuth PKCE flow: opens the browser, the user
// authorizes, copies the code from the callback page, and pastes it back.
func Login(ctx context.Context, cfg *OAuthConfig, store *TokenStore) (*Token, error) {
	pkce, err := GeneratePKCE()
	if err != nil {
		return nil, fmt.Errorf("generate PKCE: %w", err)
	}

	authURL := buildAuthURL(cfg, pkce)

	fmt.Println("Opening browser to authenticate...")
	fmt.Printf("If the browser doesn't open, visit:\n%s\n\n", authURL)
	openBrowser(authURL)

	fmt.Print("Paste the authorization code here: ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return nil, fmt.Errorf("no input received")
	}
	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return nil, fmt.Errorf("empty authorization code")
	}

	// The callback page may provide code and state separated by '#'.
	code := input
	state := ""
	if idx := strings.Index(input, "#"); idx >= 0 {
		code = input[:idx]
		state = input[idx+1:]
	}

	return exchangeCode(ctx, cfg, code, state, pkce.CodeVerifier, store)
}

func buildAuthURL(cfg *OAuthConfig, pkce *PKCEParams) string {
	v := url.Values{}
	v.Set("client_id", cfg.ClientID)
	v.Set("response_type", "code")
	v.Set("redirect_uri", cfg.RedirectURI)
	v.Set("scope", cfg.Scope)
	v.Set("code_challenge", pkce.CodeChallenge)
	v.Set("code_challenge_method", "S256")
	v.Set("state", pkce.CodeVerifier)
	for k, val := range cfg.ExtraParams {
		v.Set(k, val)
	}
	return cfg.AuthURL + "?" + v.Encode()
}

func exchangeCode(ctx context.Context, cfg *OAuthConfig, code, state, codeVerifier string, store *TokenStore) (*Token, error) {
	body := map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     cfg.ClientID,
		"code":          code,
		"state":         state,
		"redirect_uri":  cfg.RedirectURI,
		"code_verifier": codeVerifier,
	}
	return doTokenRequest(ctx, cfg.TokenURL, body, store)
}

// Refresh exchanges a refresh token for a new access token.
func Refresh(ctx context.Context, cfg *OAuthConfig, refreshToken string) (*Token, error) {
	body := map[string]string{
		"grant_type":    "refresh_token",
		"client_id":     cfg.ClientID,
		"refresh_token": refreshToken,
	}
	return doTokenRequest(ctx, cfg.TokenURL, body, nil)
}

func doTokenRequest(ctx context.Context, tokenURL string, body map[string]string, store *TokenStore) (*Token, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("token request failed (%d): %s: %s", resp.StatusCode, errResp.Error, errResp.Description)
	}

	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	token := &Token{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second),
		TokenType:    raw.TokenType,
	}

	if store != nil {
		if err := store.Save(token); err != nil {
			return nil, fmt.Errorf("save token: %w", err)
		}
	}

	return token, nil
}

func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}

	return exec.Command(cmd, args...).Start()
}
