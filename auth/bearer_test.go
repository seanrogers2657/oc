package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestBearerAuthSetsHeader(t *testing.T) {
	dir := t.TempDir()
	store := NewTokenStoreAt(filepath.Join(dir, "auth.json"))
	cfg := &OAuthConfig{
		ExtraHeaders: map[string]string{"x-custom": "value"},
	}
	token := &Token{
		AccessToken:  "sk-ant-oat01-valid",
		RefreshToken: "sk-ant-ort01-refresh",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
	}

	b := &BearerAuth{Config: cfg, Store: store, Token: token}

	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	if err := b.Authenticate(req); err != nil {
		t.Fatalf("Authenticate() = %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer sk-ant-oat01-valid" {
		t.Errorf("Authorization = %q", got)
	}
	if got := req.Header.Get("x-custom"); got != "value" {
		t.Errorf("x-custom = %q, want value", got)
	}
}

func TestBearerAuthExtraHeaderAppends(t *testing.T) {
	dir := t.TempDir()
	store := NewTokenStoreAt(filepath.Join(dir, "auth.json"))
	cfg := &OAuthConfig{
		ExtraHeaders: map[string]string{"x-beta": "oauth-flag"},
	}
	token := &Token{
		AccessToken: "tok",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}

	b := &BearerAuth{Config: cfg, Store: store, Token: token}

	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	req.Header.Set("x-beta", "existing")
	b.Authenticate(req)

	if got := req.Header.Get("x-beta"); got != "existing,oauth-flag" {
		t.Errorf("x-beta = %q, want existing,oauth-flag", got)
	}
}

func TestBearerAuthRefreshesExpiredToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["grant_type"] != "refresh_token" {
			t.Errorf("grant_type = %q, want refresh_token", body["grant_type"])
		}
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "refreshed-token",
			"refresh_token": "new-rt",
			"expires_in":    28800,
			"token_type":    "bearer",
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	store := NewTokenStoreAt(filepath.Join(dir, "auth.json"))
	cfg := &OAuthConfig{
		ClientID: "test-client",
		TokenURL: srv.URL,
	}
	token := &Token{
		AccessToken:  "expired-token",
		RefreshToken: "rt-old",
		ExpiresAt:    time.Now().Add(-1 * time.Hour),
	}

	b := &BearerAuth{Config: cfg, Store: store, Token: token}

	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	if err := b.Authenticate(req); err != nil {
		t.Fatalf("Authenticate() = %v", err)
	}

	if got := req.Header.Get("Authorization"); got != "Bearer refreshed-token" {
		t.Errorf("Authorization = %q", got)
	}
	if b.Token.AccessToken != "refreshed-token" {
		t.Errorf("Token.AccessToken = %q, want refreshed", b.Token.AccessToken)
	}

	// Token should have been persisted
	loaded, _ := store.Load()
	if loaded == nil || loaded.AccessToken != "refreshed-token" {
		t.Errorf("stored token = %v, want refreshed", loaded)
	}
}

func TestBearerAuthRefreshError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "refresh token expired",
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	store := NewTokenStoreAt(filepath.Join(dir, "auth.json"))
	cfg := &OAuthConfig{
		ClientID: "test-client",
		TokenURL: srv.URL,
	}
	token := &Token{
		AccessToken:  "expired",
		RefreshToken: "bad-refresh",
		ExpiresAt:    time.Now().Add(-1 * time.Hour),
	}

	b := &BearerAuth{Config: cfg, Store: store, Token: token}

	req, _ := http.NewRequest("GET", "https://example.com", nil)
	if err := b.Authenticate(req); err == nil {
		t.Fatal("expected error from refresh failure")
	}
}
