package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// PKCEParams holds the generated PKCE code verifier, challenge, and state.
type PKCEParams struct {
	CodeVerifier  string
	CodeChallenge string
	State         string
}

// GeneratePKCE creates a new PKCE parameter set with a 43-byte random verifier,
// SHA-256 code challenge, and random state.
func GeneratePKCE() (*PKCEParams, error) {
	verifier, err := randomBase64URL(32)
	if err != nil {
		return nil, err
	}
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])

	state, err := randomBase64URL(16)
	if err != nil {
		return nil, err
	}
	return &PKCEParams{
		CodeVerifier:  verifier,
		CodeChallenge: challenge,
		State:         state,
	}, nil
}

func randomBase64URL(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
