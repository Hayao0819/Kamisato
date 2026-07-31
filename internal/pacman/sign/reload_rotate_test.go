package sign

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// TestRotateThenReloadPersists is the high-risk check: save() uses
// SerializePrivateWithoutSigning, so a rotation's revocation of the old subkey
// and the new subkey binding must survive a round trip through disk.
func TestRotateThenReloadPersists(t *testing.T) {
	dir := t.TempDir()
	k, err := GenerateSigningKey(dir, "R", "r@example.com", 0, 365*24*time.Hour, "pw")
	if err != nil {
		t.Fatal(err)
	}
	oldSub := k.Subkeys()[0].Fingerprint
	if err := k.RotateSubkey(packet.KeySuperseded, "rot", 365*24*time.Hour, "pw"); err != nil {
		t.Fatal(err)
	}
	primary := k.PrimaryFingerprint()

	// Reload from disk with the passphrase.
	rk, err := LoadSigningKey(dir, "pw")
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if rk.PrimaryFingerprint() != primary {
		t.Fatalf("primary changed on reload")
	}

	var oldRevoked, newActive int
	for _, s := range rk.Subkeys() {
		if s.Fingerprint == oldSub {
			if !s.Revoked {
				t.Errorf("old subkey revocation LOST across reload")
			}
			oldRevoked++
		} else if s.CanSign && !s.Revoked {
			newActive++
		}
	}
	if oldRevoked != 1 || newActive != 1 {
		t.Errorf("after reload want old-revoked=1 new-active=1, got %d/%d subs=%+v", oldRevoked, newActive, rk.Subkeys())
	}

	// The reloaded key must still sign, and the signature verify against its public export.
	f := filepath.Join(dir, "x")
	_ = os.WriteFile(f, []byte("data"), 0o644)
	sig, err := rk.Sign(context.Background(), f)
	if err != nil {
		t.Fatalf("reloaded key sign: %v", err)
	}
	pub, _ := rk.PublicEntity()
	sf, _ := os.Open(f)
	df, _ := os.Open(sig)
	if _, err := openpgp.CheckDetachedSignature(openpgp.EntityList{pub}, sf, df, nil); err != nil {
		t.Fatalf("reloaded rotated key signature does not verify: %v", err)
	}
}
