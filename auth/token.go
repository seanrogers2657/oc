package auth

import (
	"time"

	"github.com/srogers/oc/config"
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

// TokenStore persists tokens within ~/.oc/config.json.
type TokenStore struct {
	path string
}

// NewTokenStore returns a store using the default path ~/.oc/config.json.
func NewTokenStore() *TokenStore {
	return &TokenStore{path: config.DefaultPath()}
}

// NewTokenStoreAt returns a store using a custom path (for testing).
func NewTokenStoreAt(path string) *TokenStore {
	return &TokenStore{path: path}
}

// Load reads the stored token from the config file.
// Returns nil, nil if the file doesn't exist or contains no token.
func (s *TokenStore) Load() (*Token, error) {
	fc, err := config.LoadFile(s.path)
	if err != nil {
		return nil, err
	}
	if fc == nil || fc.AccessToken == nil {
		return nil, nil
	}
	t := &Token{
		AccessToken: *fc.AccessToken,
	}
	if fc.RefreshToken != nil {
		t.RefreshToken = *fc.RefreshToken
	}
	if fc.ExpiresAt != nil {
		t.ExpiresAt = *fc.ExpiresAt
	}
	if fc.TokenType != nil {
		t.TokenType = *fc.TokenType
	}
	return t, nil
}

// Save writes the token into the config file, preserving other fields.
func (s *TokenStore) Save(t *Token) error {
	fc, _ := config.LoadFile(s.path)
	if fc == nil {
		fc = &config.FileConfig{}
	}
	fc.AccessToken = &t.AccessToken
	fc.RefreshToken = &t.RefreshToken
	fc.ExpiresAt = &t.ExpiresAt
	fc.TokenType = &t.TokenType
	return config.SaveFile(s.path, fc)
}

// Clear removes the token fields from the config file, preserving other fields.
func (s *TokenStore) Clear() error {
	fc, _ := config.LoadFile(s.path)
	if fc == nil {
		return nil
	}
	fc.AccessToken = nil
	fc.RefreshToken = nil
	fc.ExpiresAt = nil
	fc.TokenType = nil
	return config.SaveFile(s.path, fc)
}
