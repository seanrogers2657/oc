package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testOAuthConfig(tokenURL string) *OAuthConfig {
	return &OAuthConfig{
		ClientID:    "test-client-id",
		AuthURL:     "https://auth.example.com/authorize",
		TokenURL:    tokenURL,
		RedirectURI: "https://example.com/callback",
		Scope:       "read write",
		ExtraParams: map[string]string{"code": "true"},
	}
}

func TestRefresh(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["grant_type"] != "refresh_token" {
			t.Errorf("grant_type = %q", body["grant_type"])
		}
		if body["client_id"] != "test-client-id" {
			t.Errorf("client_id = %q", body["client_id"])
		}
		if body["refresh_token"] != "rt-123" {
			t.Errorf("refresh_token = %q", body["refresh_token"])
		}
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-at",
			"refresh_token": "new-rt",
			"expires_in":    3600,
			"token_type":    "bearer",
		})
	}))
	defer srv.Close()

	cfg := testOAuthConfig(srv.URL)
	token, err := Refresh(context.Background(), cfg, "rt-123")
	if err != nil {
		t.Fatalf("Refresh() = %v", err)
	}
	if token.AccessToken != "new-at" {
		t.Errorf("AccessToken = %q", token.AccessToken)
	}
	if token.RefreshToken != "new-rt" {
		t.Errorf("RefreshToken = %q", token.RefreshToken)
	}
}

func TestRefreshError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "token revoked",
		})
	}))
	defer srv.Close()

	cfg := testOAuthConfig(srv.URL)
	_, err := Refresh(context.Background(), cfg, "bad-token")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildAuthURL(t *testing.T) {
	cfg := &OAuthConfig{
		ClientID:    "my-client",
		AuthURL:     "https://auth.example.com/authorize",
		RedirectURI: "https://example.com/callback",
		Scope:       "read write",
		ExtraParams: map[string]string{"code": "true", "custom": "val"},
	}
	pkce := &PKCEParams{
		CodeVerifier:  "test-verifier",
		CodeChallenge: "test-challenge",
	}
	u := buildAuthURL(cfg, pkce)

	for _, want := range []string{
		"code=true",
		"custom=val",
		"client_id=my-client",
		"response_type=code",
		"redirect_uri=" + url_encode("https://example.com/callback"),
		"code_challenge=test-challenge",
		"code_challenge_method=S256",
		"state=test-verifier",
		"scope=read+write",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("auth URL missing %q:\n%s", want, u)
		}
	}

	if !strings.HasPrefix(u, cfg.AuthURL+"?") {
		t.Errorf("auth URL has wrong base: %s", u)
	}
}

func TestExchangeCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["grant_type"] != "authorization_code" {
			t.Errorf("grant_type = %q", body["grant_type"])
		}
		if body["code"] != "auth-code" {
			t.Errorf("code = %q", body["code"])
		}
		if body["state"] != "my-state" {
			t.Errorf("state = %q", body["state"])
		}
		if body["code_verifier"] != "my-verifier" {
			t.Errorf("code_verifier = %q", body["code_verifier"])
		}
		if body["redirect_uri"] != "https://example.com/callback" {
			t.Errorf("redirect_uri = %q", body["redirect_uri"])
		}
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "at-new",
			"refresh_token": "rt-new",
			"expires_in":    28800,
			"token_type":    "bearer",
		})
	}))
	defer srv.Close()

	cfg := testOAuthConfig(srv.URL)
	token, err := exchangeCode(context.Background(), cfg, "auth-code", "my-state", "my-verifier", nil)
	if err != nil {
		t.Fatalf("exchangeCode() = %v", err)
	}
	if token.AccessToken != "at-new" {
		t.Errorf("AccessToken = %q", token.AccessToken)
	}
}

func TestAnthropicOAuthPreset(t *testing.T) {
	cfg := AnthropicOAuth()
	if cfg.ClientID == "" {
		t.Error("ClientID is empty")
	}
	if cfg.AuthURL == "" {
		t.Error("AuthURL is empty")
	}
	if cfg.TokenURL == "" {
		t.Error("TokenURL is empty")
	}
	if cfg.ExtraHeaders["anthropic-beta"] != "oauth-2025-04-20" {
		t.Errorf("ExtraHeaders = %v", cfg.ExtraHeaders)
	}
}

// url_encode is a test helper matching url.QueryEscape.
func url_encode(s string) string {
	s = strings.ReplaceAll(s, " ", "+")
	s = strings.ReplaceAll(s, ":", "%3A")
	s = strings.ReplaceAll(s, "/", "%2F")
	return s
}
