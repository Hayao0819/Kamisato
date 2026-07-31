package sign

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

func genKey(t *testing.T, dir, passphrase string) *SigningKey {
	t.Helper()
	k, err := GenerateSigningKey(dir, "MyRepo", "repo@example.com", 0, 365*24*time.Hour, passphrase)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return k
}

// signAndVerify signs a scratch file with the key's subkey and verifies the
// detached signature against the key's exported public material — the end-to-end
// guarantee a keyring must provide.
func signAndVerify(t *testing.T, k *SigningKey) {
	t.Helper()
	dir := t.TempDir()
	pkg := filepath.Join(dir, "pkg.tar.zst")
	if err := os.WriteFile(pkg, []byte("payload bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	sigPath, err := k.Sign(context.Background(), pkg)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	pub, err := k.PublicEntity()
	if err != nil {
		t.Fatalf("public entity: %v", err)
	}
	if pub.PrivateKey != nil {
		t.Error("PublicEntity leaked private key material")
	}
	signed, err := os.Open(pkg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = signed.Close() }()
	sig, err := os.Open(sigPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sig.Close() }()

	signer, err := openpgp.CheckDetachedSignature(openpgp.EntityList{pub}, signed, sig, nil)
	if err != nil {
		t.Fatalf("verify against published public key: %v", err)
	}
	if signer == nil {
		t.Fatal("no signer returned")
	}
}

func TestGenerateSignsAndVerifies(t *testing.T) {
	k := genKey(t, t.TempDir(), "")
	if !k.HasPrimarySecret() {
		t.Error("freshly generated key should hold the primary secret")
	}
	// Exactly one signing subkey, no encryption subkey.
	subs := k.Subkeys()
	if len(subs) != 1 || !subs[0].CanSign {
		t.Fatalf("want one signing subkey, got %+v", subs)
	}
	signAndVerify(t, k)
}

func TestGenerateSigningKeyAllowsOnlyOneConcurrentCreator(t *testing.T) {
	dir := t.TempDir()
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, err := GenerateSigningKey(dir, "MyRepo", "repo@example.com", 0, time.Hour, "")
			results <- err
		}()
	}
	close(start)

	successes := 0
	for range 2 {
		if err := <-results; err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful concurrent creators = %d, want 1", successes)
	}
	if _, err := LoadSigningKey(dir, ""); err != nil {
		t.Fatalf("winning signing key is not readable: %v", err)
	}
}

func TestSigningKeyRejectsStaleSnapshotUpdate(t *testing.T) {
	dir := t.TempDir()
	genKey(t, dir, "")
	first, err := LoadSigningKey(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	stale, err := LoadSigningKey(dir, "")
	if err != nil {
		t.Fatal(err)
	}

	if err := first.AddSubkey(time.Hour, ""); err != nil {
		t.Fatalf("first AddSubkey: %v", err)
	}
	if err := stale.AddSubkey(time.Hour, ""); !errors.Is(err, errSigningKeyChanged) {
		t.Fatalf("stale AddSubkey error = %v, want signing-key conflict", err)
	}
	reloaded, err := LoadSigningKey(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := len(reloaded.Subkeys()); got != 2 {
		t.Fatalf("persisted subkeys = %d, want original + first update", got)
	}
}

func TestReloadWithPassphrase(t *testing.T) {
	dir := t.TempDir()
	k := genKey(t, dir, "s3cr3t")
	fpr := k.PrimaryFingerprint()

	if _, err := LoadSigningKey(dir, "wrong"); err == nil {
		t.Error("load with wrong passphrase should fail")
	}
	reloaded, err := LoadSigningKey(dir, "s3cr3t")
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.PrimaryFingerprint() != fpr {
		t.Error("fingerprint changed across reload")
	}
	signAndVerify(t, reloaded)
}

func TestRotateKeepsPrimaryFingerprint(t *testing.T) {
	dir := t.TempDir()
	k := genKey(t, dir, "")
	before := k.PrimaryFingerprint()
	oldSub := k.Subkeys()[0].Fingerprint

	if err := k.RotateSubkey(packet.KeySuperseded, "annual rotation", 365*24*time.Hour, ""); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if k.PrimaryFingerprint() != before {
		t.Error("rotation must not change the primary fingerprint (the trust anchor)")
	}

	var oldRevoked, newActive int
	for _, s := range k.Subkeys() {
		if s.Fingerprint == oldSub {
			if !s.Revoked {
				t.Error("old subkey should be revoked after rotation")
			}
			oldRevoked++
		} else if s.CanSign && !s.Revoked {
			newActive++
		}
	}
	if oldRevoked != 1 || newActive != 1 {
		t.Errorf("want old revoked + one new active, got old=%d new=%d subs=%+v", oldRevoked, newActive, k.Subkeys())
	}
	// The new subkey must sign and verify.
	signAndVerify(t, k)
}

func TestRevokeSubkeyByFingerprint(t *testing.T) {
	k := genKey(t, t.TempDir(), "")
	sub := k.Subkeys()[0].Fingerprint
	if err := k.RevokeSubkey(sub, packet.KeyCompromised, "leaked", ""); err != nil {
		t.Fatalf("revoke subkey: %v", err)
	}
	if !k.Subkeys()[0].Revoked {
		t.Error("subkey should be revoked")
	}
	if err := k.RevokeSubkey("DEADBEEF", packet.NoReason, "", ""); err == nil {
		t.Error("revoking an unknown fingerprint should error")
	}
}

func TestRevokePrimary(t *testing.T) {
	k := genKey(t, t.TempDir(), "")
	if err := k.RevokePrimary(packet.KeyCompromised, "primary leaked", ""); err != nil {
		t.Fatalf("revoke primary: %v", err)
	}
	if !k.Revoked() {
		t.Error("primary should be revoked")
	}
	// The exported public key must carry the revocation so it propagates.
	pub, err := k.PublicEntity()
	if err != nil {
		t.Fatal(err)
	}
	if !pub.Revoked(time.Now()) {
		t.Error("exported public key must carry the primary revocation")
	}
}

func TestReasonMapping(t *testing.T) {
	cases := map[string]struct {
		reason packet.ReasonForRevocation
		hard   bool
	}{
		"superseded":  {packet.KeySuperseded, false},
		"retired":     {packet.KeyRetired, false},
		"compromised": {packet.KeyCompromised, true},
		"unspecified": {packet.NoReason, true},
	}
	for word, want := range cases {
		got, err := ParseRevocationReason(word)
		if err != nil {
			t.Fatalf("%s: %v", word, err)
		}
		if got != want.reason {
			t.Errorf("%s: reason = %v, want %v", word, got, want.reason)
		}
		if IsHardRevocation(got) != want.hard {
			t.Errorf("%s: hard = %v, want %v", word, IsHardRevocation(got), want.hard)
		}
	}
	if _, err := ParseRevocationReason("bogus"); err == nil {
		t.Error("unknown reason should error")
	}
}
