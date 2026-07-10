package http

import (
	"strings"
	"testing"
)

func TestGenerateSessionTokenIsOpaqueAndNotMockToken(t *testing.T) {
	token := generateSessionToken()
	if !strings.HasPrefix(token, "fnr_") {
		t.Fatalf("expected fnr_ token prefix, got %q", token)
	}
	if strings.HasPrefix(token, "mock_token_") {
		t.Fatal("session token must not use forgeable mock token format")
	}
	if len(token) != 68 {
		t.Fatalf("expected 68 character token, got %d", len(token))
	}
}

func TestHashSessionToken(t *testing.T) {
	hash := hashSessionToken("fnr_test")
	if len(hash) != 64 {
		t.Fatalf("expected 64 character sha256 hex hash, got %d", len(hash))
	}
	if hash == "fnr_test" {
		t.Fatal("token hash must not store the raw token")
	}
	if hash != hashSessionToken("fnr_test") {
		t.Fatal("token hash must be stable")
	}
}
