package auth

import (
	"encoding/base64"
	"testing"
)

func TestNewOpaqueToken(t *testing.T) {
	if _, err := NewOpaqueToken(0); err == nil {
		t.Fatal("zero-length token must be rejected")
	}

	first, err := NewOpaqueToken(32)
	if err != nil {
		t.Fatalf("NewOpaqueToken: %v", err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(first)
	if err != nil {
		t.Fatalf("token is not base64url: %v", err)
	}
	if len(raw) != 32 {
		t.Fatalf("decoded length = %d, want 32", len(raw))
	}

	second, err := NewOpaqueToken(32)
	if err != nil {
		t.Fatalf("second NewOpaqueToken: %v", err)
	}
	if first == second {
		t.Fatal("independent random tokens must differ")
	}
}
