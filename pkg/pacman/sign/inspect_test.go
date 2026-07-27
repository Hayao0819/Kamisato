package sign_test

import (
	"testing"

	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

func TestInspectDetached(t *testing.T) {
	t.Parallel()
	signer := newTestEntity(t, "inspector")
	payload := []byte("package bytes")
	sig := detachSign(t, signer, payload)

	info, err := sign.InspectDetached(sig)
	if err != nil {
		t.Fatalf("InspectDetached: %v", err)
	}
	wantFpr := upperFingerprint(signer)
	if info.Fingerprint != wantFpr {
		t.Errorf("fingerprint = %q, want %q", info.Fingerprint, wantFpr)
	}
	if info.KeyID == "" || len(info.KeyID) != 16 {
		t.Errorf("key id = %q, want 16 hex chars", info.KeyID)
	}
	if info.PubKeyAlgo != "EdDSA" {
		t.Errorf("pubkey algo = %q, want EdDSA", info.PubKeyAlgo)
	}
	if info.Hash == "" {
		t.Error("hash is empty")
	}
	if info.CreatedAt.IsZero() {
		t.Error("created at is zero")
	}
}

func TestInspectDetached_Garbage(t *testing.T) {
	t.Parallel()
	if _, err := sign.InspectDetached([]byte("not a signature")); err == nil {
		t.Fatal("InspectDetached accepted garbage input")
	}
}
