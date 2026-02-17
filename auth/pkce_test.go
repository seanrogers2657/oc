package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestGeneratePKCE(t *testing.T) {
	p, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE() error = %v", err)
	}

	// Verifier should be a base64url-encoded 32-byte value (43 chars)
	if len(p.CodeVerifier) != 43 {
		t.Errorf("CodeVerifier length = %d, want 43", len(p.CodeVerifier))
	}

	// Challenge should be SHA-256 of verifier, base64url-encoded
	h := sha256.Sum256([]byte(p.CodeVerifier))
	expected := base64.RawURLEncoding.EncodeToString(h[:])
	if p.CodeChallenge != expected {
		t.Errorf("CodeChallenge = %q, want SHA-256 of verifier", p.CodeChallenge)
	}

	// State should be non-empty
	if p.State == "" {
		t.Error("State is empty")
	}

	// Two calls should produce different values
	p2, _ := GeneratePKCE()
	if p.CodeVerifier == p2.CodeVerifier {
		t.Error("two calls produced same CodeVerifier")
	}
	if p.State == p2.State {
		t.Error("two calls produced same State")
	}
}
