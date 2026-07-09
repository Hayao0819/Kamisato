package sign_test

import (
	"bytes"
	"crypto"
	_ "crypto/sha1" // register SHA-1 so a forged SHA-1 signature can be built
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"

	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

// serializeArmored writes the entity's public part as an ASCII-armored keyring.
func serializeArmored(buf *bytes.Buffer, e *openpgp.Entity) error {
	w, err := armor.Encode(buf, openpgp.PublicKeyType, nil)
	if err != nil {
		return err
	}
	if err := e.Serialize(w); err != nil {
		return err
	}
	return w.Close()
}

// spaceEvery4 inserts a space every four characters, mimicking how gpg prints
// fingerprints, to exercise the keyring's normalization.
func spaceEvery4(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && i%4 == 0 {
			b.WriteRune(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// edConfig produces fast EdDSA (Curve25519) keys so the tests stay quick.
func edConfig() *packet.Config {
	return &packet.Config{Algorithm: packet.PubKeyAlgoEdDSA}
}

func newTestEntity(t *testing.T, name string) *openpgp.Entity {
	t.Helper()
	e, err := openpgp.NewEntity(name, "test", name+"@example.com", edConfig())
	if err != nil {
		t.Fatalf("NewEntity(%s): %v", name, err)
	}
	return e
}

// upperFingerprint mirrors the fingerprint formatting VerifyDetached returns.
func upperFingerprint(e *openpgp.Entity) string {
	var b strings.Builder
	for _, x := range e.PrimaryKey.Fingerprint {
		b.WriteString(strings.ToUpper(hexByte(x)))
	}
	return b.String()
}

func hexByte(b byte) string {
	const hex = "0123456789abcdef"
	return string([]byte{hex[b>>4], hex[b&0x0f]})
}

// writePubKeyFile serializes the public part of entities to a binary keyring file.
func writePubKeyFile(t *testing.T, entities ...*openpgp.Entity) string {
	t.Helper()
	var buf bytes.Buffer
	for _, e := range entities {
		if err := e.Serialize(&buf); err != nil {
			t.Fatalf("serialize pubkey: %v", err)
		}
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "keyring.gpg")
	if err := os.WriteFile(p, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write keyring: %v", err)
	}
	return p
}

// detachSign produces a binary (non-armored) detached signature of payload.
func detachSign(t *testing.T, signer *openpgp.Entity, payload []byte) []byte {
	t.Helper()
	return detachSignWith(t, signer, payload, edConfig())
}

// detachSignWith is detachSign with an explicit config (key/hash selection).
func detachSignWith(t *testing.T, signer *openpgp.Entity, payload []byte, cfg *packet.Config) []byte {
	t.Helper()
	var sig bytes.Buffer
	if err := openpgp.DetachSign(&sig, signer, bytes.NewReader(payload), cfg); err != nil {
		t.Fatalf("DetachSign: %v", err)
	}
	return sig.Bytes()
}

func TestVerifyDetached_Valid(t *testing.T) {
	signer := newTestEntity(t, "signer")
	payload := []byte("a binary package payload")
	sig := detachSign(t, signer, payload)

	keyPath := writePubKeyFile(t, signer)
	kr, err := sign.LoadKeyring(keyPath, nil)
	if err != nil {
		t.Fatalf("LoadKeyring: %v", err)
	}

	fpr, err := kr.VerifyDetached(bytes.NewReader(payload), bytes.NewReader(sig))
	if err != nil {
		t.Fatalf("VerifyDetached: %v", err)
	}
	if want := upperFingerprint(signer); fpr != want {
		t.Errorf("fingerprint = %q, want %q", fpr, want)
	}
}

func TestVerifyDetached_TamperedPayload(t *testing.T) {
	signer := newTestEntity(t, "signer")
	payload := []byte("original payload")
	sig := detachSign(t, signer, payload)

	keyPath := writePubKeyFile(t, signer)
	kr, err := sign.LoadKeyring(keyPath, nil)
	if err != nil {
		t.Fatalf("LoadKeyring: %v", err)
	}

	tampered := []byte("ORIGINAL payload (modified)")
	if _, err := kr.VerifyDetached(bytes.NewReader(tampered), bytes.NewReader(sig)); err == nil {
		t.Fatal("expected error for tampered payload, got nil")
	}
}

func TestVerifyDetached_SignerNotInKeyring(t *testing.T) {
	signer := newTestEntity(t, "signer")
	other := newTestEntity(t, "other")
	payload := []byte("payload")
	sig := detachSign(t, signer, payload)

	// keyring only contains "other", not the actual signer.
	keyPath := writePubKeyFile(t, other)
	kr, err := sign.LoadKeyring(keyPath, nil)
	if err != nil {
		t.Fatalf("LoadKeyring: %v", err)
	}

	if _, err := kr.VerifyDetached(bytes.NewReader(payload), bytes.NewReader(sig)); err == nil {
		t.Fatal("expected ErrUnknownIssuer-style rejection, got nil")
	}
}

func TestVerifyDetached_NotInTrustedAllowlist(t *testing.T) {
	signer := newTestEntity(t, "signer")
	payload := []byte("payload")
	sig := detachSign(t, signer, payload)

	keyPath := writePubKeyFile(t, signer)
	// allowlist a different fingerprint than the signer's.
	kr, err := sign.LoadKeyring(keyPath, []string{"DEAD BEEF DEAD BEEF DEAD BEEF DEAD BEEF DEAD BEEF"})
	if err != nil {
		t.Fatalf("LoadKeyring: %v", err)
	}

	if _, err := kr.VerifyDetached(bytes.NewReader(payload), bytes.NewReader(sig)); err == nil {
		t.Fatal("expected rejection for key absent from trusted allowlist, got nil")
	}
}

func TestVerifyDetached_TrustedAllowlistMatch(t *testing.T) {
	signer := newTestEntity(t, "signer")
	payload := []byte("payload")
	sig := detachSign(t, signer, payload)

	keyPath := writePubKeyFile(t, signer)
	// supply the signer's fingerprint with spaces and lowercase to exercise
	// normalization.
	spaced := spaceEvery4(strings.ToLower(upperFingerprint(signer)))
	kr, err := sign.LoadKeyring(keyPath, []string{spaced})
	if err != nil {
		t.Fatalf("LoadKeyring: %v", err)
	}

	fpr, err := kr.VerifyDetached(bytes.NewReader(payload), bytes.NewReader(sig))
	if err != nil {
		t.Fatalf("VerifyDetached with matching allowlist: %v", err)
	}
	if fpr != upperFingerprint(signer) {
		t.Errorf("fingerprint = %q, want %q", fpr, upperFingerprint(signer))
	}
}

func TestVerifyDetached_ExpiredKey(t *testing.T) {
	// A key created in the past with a short lifetime is expired by now.
	cfg := edConfig()
	past := time.Now().Add(-48 * time.Hour)
	cfg.Time = func() time.Time { return past }
	cfg.KeyLifetimeSecs = 3600 // 1 hour, long expired by the time we verify

	signer, err := openpgp.NewEntity("expired", "test", "expired@example.com", cfg)
	if err != nil {
		t.Fatalf("NewEntity expired: %v", err)
	}
	payload := []byte("payload")
	var sigBuf bytes.Buffer
	if err := openpgp.DetachSign(&sigBuf, signer, bytes.NewReader(payload), cfg); err != nil {
		t.Fatalf("DetachSign expired: %v", err)
	}

	keyPath := writePubKeyFile(t, signer)
	kr, err := sign.LoadKeyring(keyPath, nil)
	if err != nil {
		t.Fatalf("LoadKeyring: %v", err)
	}

	if _, err := kr.VerifyDetached(bytes.NewReader(payload), bytes.NewReader(sigBuf.Bytes())); err == nil {
		t.Fatal("expected rejection for expired key, got nil")
	}
}

func TestLoadKeyring_Armored(t *testing.T) {
	signer := newTestEntity(t, "signer")
	payload := []byte("payload")
	sig := detachSign(t, signer, payload)

	// write an armored public keyring.
	var buf bytes.Buffer
	if err := serializeArmored(&buf, signer); err != nil {
		t.Fatalf("serialize armored: %v", err)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "key.asc")
	if err := os.WriteFile(p, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write armored: %v", err)
	}

	kr, err := sign.LoadKeyring(p, nil)
	if err != nil {
		t.Fatalf("LoadKeyring armored: %v", err)
	}
	if _, err := kr.VerifyDetached(bytes.NewReader(payload), bytes.NewReader(sig)); err != nil {
		t.Fatalf("VerifyDetached after armored load: %v", err)
	}
}

// rsaConfig produces a V4 RSA signing key. RSA (unlike EdDSA) permits a SHA-1
// digest, so it is used to forge the downgraded signature below. Randomized-salt
// notation is disabled because its salt-length lookup itself refuses SHA-1,
// which would mask the verifier-side rejection we want to exercise.
func rsaConfig() *packet.Config {
	deterministic := false
	return &packet.Config{
		Algorithm:                             packet.PubKeyAlgoRSA,
		RSABits:                               2048,
		NonDeterministicSignaturesViaNotation: &deterministic,
	}
}

// forgeSHA1DetachedSig builds a V4 detached binary signature over payload using
// crypto.SHA1, bypassing the normal hash-selection path that would otherwise
// upgrade the digest. It mirrors openpgp.detachSign but pins Hash=SHA1.
func forgeSHA1DetachedSig(t *testing.T, signer *openpgp.Entity, payload []byte) []byte {
	t.Helper()
	cfg := rsaConfig()
	sig := &packet.Signature{
		Version:      signer.PrimaryKey.Version,
		SigType:      packet.SigTypeBinary,
		PubKeyAlgo:   signer.PrimaryKey.PubKeyAlgo,
		Hash:         crypto.SHA1,
		CreationTime: signer.PrimaryKey.CreationTime,
		IssuerKeyId:  &signer.PrimaryKey.KeyId,
	}
	h, err := sig.PrepareSign(cfg)
	if err != nil {
		t.Fatalf("PrepareSign: %v", err)
	}
	// Binary signature: the message is hashed directly (no canonical-text wrap).
	if _, err := h.Write(payload); err != nil {
		t.Fatalf("hash payload: %v", err)
	}
	if err := sig.Sign(h, signer.PrivateKey, cfg); err != nil {
		t.Fatalf("Sign (SHA-1 forge): %v", err)
	}
	var buf bytes.Buffer
	if err := sig.Serialize(&buf); err != nil {
		t.Fatalf("serialize forged sig: %v", err)
	}
	return buf.Bytes()
}

// TestVerifyDetached_RejectsSHA1 forges a SHA-1 detached signature from a key
// that IS present in the keyring and asserts VerifyDetached rejects it: the
// digest downgrade must be refused before the signer is even resolved. The
// companion SHA-256 case below confirms a strong digest from the same key is
// accepted, so the rejection is the digest's doing, not a missing key.
func TestVerifyDetached_RejectsSHA1(t *testing.T) {
	signer, err := openpgp.NewEntity("rsa-signer", "test", "rsa@example.com", rsaConfig())
	if err != nil {
		t.Fatalf("NewEntity rsa: %v", err)
	}
	payload := []byte("a binary package payload")

	keyPath := writePubKeyFile(t, signer)
	kr, err := sign.LoadKeyring(keyPath, nil)
	if err != nil {
		t.Fatalf("LoadKeyring: %v", err)
	}

	// SHA-256 from the same RSA key is accepted (sanity: the key is trusted).
	good := detachSignWith(t, signer, payload, rsaConfig())
	if _, err := kr.VerifyDetached(bytes.NewReader(payload), bytes.NewReader(good)); err != nil {
		t.Fatalf("SHA-256 signature from a trusted key must verify: %v", err)
	}

	// SHA-1 from the SAME trusted key must be rejected (algorithm downgrade).
	forged := forgeSHA1DetachedSig(t, signer, payload)
	if _, err := kr.VerifyDetached(bytes.NewReader(payload), bytes.NewReader(forged)); err == nil {
		t.Fatal("expected rejection of a SHA-1 detached signature, got nil")
	}
}

func TestLoadKeyring_Errors(t *testing.T) {
	if _, err := sign.LoadKeyring(filepath.Join(t.TempDir(), "nope.gpg"), nil); err == nil {
		t.Error("expected error for missing keyring file")
	}
	empty := filepath.Join(t.TempDir(), "empty.gpg")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := sign.LoadKeyring(empty, nil); err == nil {
		t.Error("expected error for empty keyring file")
	}
}
