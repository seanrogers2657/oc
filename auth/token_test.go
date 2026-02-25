package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTokenValid(t *testing.T) {
	tests := []struct {
		name  string
		token *Token
		want  bool
	}{
		{"nil token", nil, false},
		{"empty access token", &Token{}, false},
		{"expired", &Token{AccessToken: "tok", ExpiresAt: time.Now().Add(-1 * time.Hour)}, false},
		{"expires within 5min", &Token{AccessToken: "tok", ExpiresAt: time.Now().Add(3 * time.Minute)}, false},
		{"valid", &Token{AccessToken: "tok", ExpiresAt: time.Now().Add(1 * time.Hour)}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.token.Valid(); got != tt.want {
				t.Errorf("Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTokenStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewTokenStoreAt(filepath.Join(dir, "config.json"))

	// Load from nonexistent file returns nil, nil
	tok, err := store.Load()
	if err != nil || tok != nil {
		t.Fatalf("Load() = %v, %v; want nil, nil", tok, err)
	}

	// Save and load
	token := &Token{
		AccessToken:  "sk-ant-oat01-test",
		RefreshToken: "sk-ant-ort01-test",
		ExpiresAt:    time.Now().Add(8 * time.Hour).Truncate(time.Second),
		TokenType:    "bearer",
	}
	if err := store.Save(token); err != nil {
		t.Fatalf("Save() = %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if loaded.AccessToken != token.AccessToken {
		t.Errorf("AccessToken = %q, want %q", loaded.AccessToken, token.AccessToken)
	}
	if loaded.RefreshToken != token.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", loaded.RefreshToken, token.RefreshToken)
	}

	// Verify file permissions
	info, err := os.Stat(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("file perm = %o, want 0600", perm)
	}
}

func TestTokenStoreClear(t *testing.T) {
	dir := t.TempDir()
	store := NewTokenStoreAt(filepath.Join(dir, "config.json"))

	// Clear nonexistent is fine
	if err := store.Clear(); err != nil {
		t.Fatalf("Clear() on empty = %v", err)
	}

	// Save then clear
	store.Save(&Token{AccessToken: "tok"})
	if err := store.Clear(); err != nil {
		t.Fatalf("Clear() = %v", err)
	}

	tok, err := store.Load()
	if err != nil || tok != nil {
		t.Fatalf("after Clear, Load() = %v, %v; want nil, nil", tok, err)
	}
}

func TestTokenStorePreservesOtherConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Write a config file with provider set
	os.WriteFile(path, []byte(`{"provider": "anthropic", "model": "claude-sonnet-4-20250514"}`), 0600)

	store := NewTokenStoreAt(path)
	store.Save(&Token{
		AccessToken:  "tok",
		RefreshToken: "rt",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		TokenType:    "bearer",
	})

	// Read raw file and verify provider is still there
	data, _ := os.ReadFile(path)
	content := string(data)
	if !contains(content, `"provider"`) {
		t.Errorf("Save clobbered provider field: %s", content)
	}
	if !contains(content, `"access_token"`) {
		t.Errorf("Save didn't write access_token: %s", content)
	}

	// Clear token and verify provider survives
	store.Clear()
	data, _ = os.ReadFile(path)
	content = string(data)
	if !contains(content, `"provider"`) {
		t.Errorf("Clear clobbered provider field: %s", content)
	}
	if contains(content, `"access_token"`) {
		t.Errorf("Clear didn't remove access_token: %s", content)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
