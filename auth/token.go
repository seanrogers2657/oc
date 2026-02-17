package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Token holds OAuth access and refresh tokens.
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

// Valid reports whether the token exists and won't expire within 5 minutes.
func (t *Token) Valid() bool {
	if t == nil || t.AccessToken == "" {
		return false
	}
	return time.Now().Add(5 * time.Minute).Before(t.ExpiresAt)
}

// TokenStore persists tokens to ~/.oc/auth.json.
type TokenStore struct {
	path string
}

// NewTokenStore returns a store using the default path ~/.oc/auth.json.
func NewTokenStore() *TokenStore {
	home, _ := os.UserHomeDir()
	return &TokenStore{path: filepath.Join(home, ".oc", "auth.json")}
}

// NewTokenStoreAt returns a store using a custom path (for testing).
func NewTokenStoreAt(path string) *TokenStore {
	return &TokenStore{path: path}
}

// Load reads the stored token. Returns nil, nil if the file doesn't exist.
func (s *TokenStore) Load() (*Token, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var t Token
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// Save writes the token to disk, creating the directory with mode 0700
// and the file with mode 0600.
func (s *TokenStore) Save(t *Token) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}

// Clear deletes the stored token file.
func (s *TokenStore) Clear() error {
	err := os.Remove(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
