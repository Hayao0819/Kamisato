package cmd

import (
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/apikey"
)

func TestGenerateAPIKeyRoundTrip(t *testing.T) {
	k, err := generateAPIKey()
	if err != nil {
		t.Fatalf("generateAPIKey: %v", err)
	}
	if !strings.HasPrefix(k, "miko_") {
		t.Errorf("key %q lacks the miko_ prefix", k)
	}

	// A freshly generated key validates against a verifier configured with it.
	v := apikey.NewVerifier([]string{k})
	if !v.Valid(k) {
		t.Error("generated key did not validate against its own verifier")
	}

	other, err := generateAPIKey()
	if err != nil {
		t.Fatalf("generateAPIKey (second): %v", err)
	}
	if k == other {
		t.Error("two generated keys collided; generation is not random")
	}
	if v.Valid(other) {
		t.Error("an unrelated key validated against the verifier")
	}
}
