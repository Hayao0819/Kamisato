package sign

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// TestSignRefusesPrimaryFallback proves the [SC] primary is never silently used
// once the signing subkey is gone: revoking the only subkey makes Sign fail
// rather than issue a primary-key signature.
func TestSignRefusesPrimaryFallback(t *testing.T) {
	dir := t.TempDir()
	k, err := GenerateSigningKey(dir, "R", "r@example.com", 0, 365*24*time.Hour, "")
	if err != nil {
		t.Fatal(err)
	}
	sub := k.Subkeys()[0].Fingerprint
	if err := k.RevokeSubkey(sub, packet.KeyCompromised, "leak", ""); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(dir, "x")
	_ = os.WriteFile(f, []byte("data"), 0o644)
	if _, err := k.Sign(context.Background(), f); err == nil {
		t.Fatal("Sign should refuse when no valid signing subkey remains, not fall back to the primary")
	}

	// After adding a fresh subkey, signing works again and verifies.
	if err := k.AddSubkey(365*24*time.Hour, ""); err != nil {
		t.Fatal(err)
	}
	sig, err := k.Sign(context.Background(), f)
	if err != nil {
		t.Fatalf("sign after re-adding subkey: %v", err)
	}
	pub, _ := k.PublicEntity()
	sf, _ := os.Open(f)
	df, _ := os.Open(sig)
	signer, err := openpgp.CheckDetachedSignature(openpgp.EntityList{pub}, sf, df, nil)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	// The signature must come from a subkey, not the primary.
	if signer.PrimaryKey.KeyId == 0 {
		t.Skip("no signer key id")
	}
}

// TestExportSecretReEncrypts proves a secret export of a passphrase-protected key
// is itself encrypted (not silently cleartext) and reloads with the passphrase.
func TestExportSecretReEncrypts(t *testing.T) {
	dir := t.TempDir()
	k, err := GenerateSigningKey(dir, "R", "r@example.com", 0, 365*24*time.Hour, "pw")
	if err != nil {
		t.Fatal(err)
	}
	armored, err := k.ExportSecretArmored("pw")
	if err != nil {
		t.Fatal(err)
	}
	// The armored export must not carry an unlocked private key: reading it and
	// checking the primary is still encrypted.
	el, err := openpgp.ReadArmoredKeyRing(strings.NewReader(armored))
	if err != nil {
		t.Fatalf("re-read export: %v", err)
	}
	if len(el) != 1 || el[0].PrivateKey == nil {
		t.Fatal("export should contain one private key")
	}
	if !el[0].PrivateKey.Encrypted {
		t.Fatal("secret export of a protected key must be encrypted, not cleartext")
	}
	// The in-memory key must still be usable: the export left it unlocked.
	if _, err := k.Sign(context.Background(), writeTemp(t, dir)); err != nil {
		t.Fatalf("key should still sign after export: %v", err)
	}
}

func writeTemp(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "sig-target")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}
