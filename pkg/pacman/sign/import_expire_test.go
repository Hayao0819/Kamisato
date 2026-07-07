package sign

import (
	"context"
	"crypto"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// TestImportRoundTrip adopts a key exported from one keystore into another and
// confirms it loads, keeps its fingerprint, and can sign.
func TestImportRoundTrip(t *testing.T) {
	src := t.TempDir()
	orig, err := GenerateSigningKey(src, "R", "r@example.com", 0, 365*24*time.Hour, "pw")
	if err != nil {
		t.Fatal(err)
	}
	fpr := orig.PrimaryFingerprint()
	armored, err := orig.ExportSecretArmored("pw")
	if err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	imported, err := ImportSigningKey(dst, strings.NewReader(armored), "pw", false)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if imported.PrimaryFingerprint() != fpr {
		t.Errorf("fingerprint changed on import: %s vs %s", imported.PrimaryFingerprint(), fpr)
	}
	// Importing again without --force must refuse.
	if _, err := ImportSigningKey(dst, strings.NewReader(armored), "pw", false); err == nil {
		t.Error("second import without force should fail")
	}
	// It reloads and signs.
	reloaded, err := LoadSigningKey(dst, "pw")
	if err != nil {
		t.Fatalf("reload imported: %v", err)
	}
	f := filepath.Join(dst, "x")
	_ = os.WriteFile(f, []byte("data"), 0o644)
	if _, err := reloaded.Sign(context.Background(), f); err != nil {
		t.Fatalf("imported key cannot sign: %v", err)
	}
}

// TestImportPreservesEncryptionSubkey confirms a legacy key with an encryption
// subkey keeps it through import and a subsequent subkey add (drop is
// generation-only now).
func TestImportPreservesEncryptionSubkey(t *testing.T) {
	// Build an entity the classic way: NewEntity adds an encryption subkey.
	entity, err := openpgp.NewEntity("Legacy", "", "legacy@example.com", &packet.Config{Algorithm: packet.PubKeyAlgoEdDSA, DefaultHash: crypto.SHA256})
	if err != nil {
		t.Fatal(err)
	}
	if err := entity.AddSigningSubkey(&packet.Config{Algorithm: packet.PubKeyAlgoEdDSA, DefaultHash: crypto.SHA256}); err != nil {
		t.Fatal(err)
	}
	countEnc := func(e *openpgp.Entity) int {
		n := 0
		for i := range e.Subkeys {
			if e.Subkeys[i].Sig != nil && e.Subkeys[i].Sig.FlagEncryptStorage {
				n++
			}
		}
		return n
	}
	if countEnc(entity) == 0 {
		t.Fatal("test setup: expected an encryption subkey")
	}

	dir := t.TempDir()
	sk := &SigningKey{dir: dir, entity: entity}
	if err := sk.save(""); err != nil {
		t.Fatal(err)
	}
	imported, err := LoadSigningKey(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if countEnc(imported.entity) == 0 {
		t.Error("import dropped the encryption subkey")
	}
	// Adding a signing subkey must not drop the encryption subkey.
	if err := imported.AddSubkey(365*24*time.Hour, ""); err != nil {
		t.Fatal(err)
	}
	if countEnc(imported.entity) == 0 {
		t.Error("AddSubkey dropped the encryption subkey")
	}
}

// TestSetExpiryExtendsPrimary confirms expiry extension re-signs the primary and
// the new expiry survives a reload.
func TestSetExpiryExtendsPrimary(t *testing.T) {
	dir := t.TempDir()
	// Start with a short-lived primary (1 hour).
	k, err := GenerateSigningKey(dir, "R", "r@example.com", time.Hour, 365*24*time.Hour, "")
	if err != nil {
		t.Fatal(err)
	}
	before := k.PrimaryExpiry()
	if before.IsZero() {
		t.Fatal("expected a primary expiry")
	}

	if err := k.SetExpiry(5*365*24*time.Hour, ExpireTargets{Primary: true, AllSubkeys: true}, ""); err != nil {
		t.Fatalf("set expiry: %v", err)
	}
	reloaded, err := LoadSigningKey(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	after := reloaded.PrimaryExpiry()
	if !after.After(before.Add(3 * 365 * 24 * time.Hour)) {
		t.Errorf("primary expiry not extended: before=%v after=%v", before, after)
	}
	// The key must still sign after re-dating.
	f := filepath.Join(dir, "x")
	_ = os.WriteFile(f, []byte("d"), 0o644)
	if _, err := reloaded.Sign(context.Background(), f); err != nil {
		t.Fatalf("sign after expiry extend: %v", err)
	}
}

// TestSetExpirySubkeyOnly confirms extending a single subkey by fingerprint moves
// its expiry out and leaves the primary untouched; the renewed subkey still signs.
func TestSetExpirySubkeyOnly(t *testing.T) {
	dir := t.TempDir()
	k, err := GenerateSigningKey(dir, "R", "r@example.com", 0, time.Hour, "")
	if err != nil {
		t.Fatal(err)
	}
	sub := k.Subkeys()[0].Fingerprint
	before := k.Subkeys()[0].Expires

	if err := k.SetExpiry(5*365*24*time.Hour, ExpireTargets{Subkey: sub}, ""); err != nil {
		t.Fatalf("extend subkey: %v", err)
	}
	if err := k.SetExpiry(time.Hour, ExpireTargets{Subkey: "DEADBEEF"}, ""); err == nil {
		t.Error("extending an unknown subkey should fail")
	}

	reloaded, err := LoadSigningKey(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	after := reloaded.Subkeys()[0].Expires
	if !after.After(before.Add(3 * 365 * 24 * time.Hour)) {
		t.Errorf("subkey expiry not extended: before=%v after=%v", before, after)
	}
	if !reloaded.PrimaryExpiry().IsZero() {
		t.Error("primary expiry should be unchanged (never) when only a subkey is targeted")
	}
	f := filepath.Join(dir, "x")
	_ = os.WriteFile(f, []byte("d"), 0o644)
	if _, err := reloaded.Sign(context.Background(), f); err != nil {
		t.Fatalf("sign after subkey renew: %v", err)
	}
}
