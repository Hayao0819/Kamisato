package gpg

import (
	"bytes"
	"crypto"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// Keyring is the trust root used to verify detached package signatures. entities
// is the set of public keys loaded from disk; trusted is an optional allowlist of
// uppercased, space-stripped primary-key fingerprints. An empty trusted map means
// "accept any key present in the keyring".
type Keyring struct {
	entities openpgp.EntityList
	trusted  map[string]bool
}

// normalizeFingerprint uppercases and strips all whitespace so fingerprints from
// config (often spaced in groups of four) compare equal to the hex we derive from
// a key.
func normalizeFingerprint(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// ReadEntities parses OpenPGP public keys from armored or binary bytes.
func ReadEntities(data []byte) (openpgp.EntityList, error) {
	if bytes.HasPrefix(data, []byte("-----BEGIN PGP")) {
		return openpgp.ReadArmoredKeyRing(bytes.NewReader(data))
	}
	return openpgp.ReadKeyRing(bytes.NewReader(data))
}

// NewKeyring builds a verifier over entities, with an optional fingerprint
// allowlist (empty means accept any entity present).
func NewKeyring(entities openpgp.EntityList, trustedFprs []string) *Keyring {
	trusted := make(map[string]bool, len(trustedFprs))
	for _, fpr := range trustedFprs {
		if n := normalizeFingerprint(fpr); n != "" {
			trusted[n] = true
		}
	}
	return &Keyring{entities: entities, trusted: trusted}
}

// LoadKeyring reads the public-key file at pubkeyPath (armored or binary). It is
// the trust root for verification, kept separate from the signing private key.
func LoadKeyring(pubkeyPath string, trustedFprs []string) (*Keyring, error) {
	data, err := os.ReadFile(pubkeyPath)
	if err != nil {
		return nil, fmt.Errorf("read keyring %q: %w", pubkeyPath, err)
	}
	entities, err := ReadEntities(data)
	if err != nil {
		return nil, fmt.Errorf("parse keyring %q: %w", pubkeyPath, err)
	}
	if len(entities) == 0 {
		return nil, fmt.Errorf("keyring %q contains no public keys", pubkeyPath)
	}
	return NewKeyring(entities, trustedFprs), nil
}

// allowedHashes pins the digest algorithms accepted on a detached signature.
// Restricting to SHA-256/384/512 rejects a downgrade to the broken SHA-1 (and
// other weak digests) before the signer is resolved, closing the signature
// algorithm-substitution attack.
var allowedHashes = []crypto.Hash{crypto.SHA256, crypto.SHA384, crypto.SHA512}

// VerifyDetached verifies a binary detached signature (sig) against the package
// bytes (signed). It fails closed: a signature over a non-allowlisted digest
// (e.g. SHA-1), any verification error, an expired or revoked signing key, or a
// key absent from a non-empty trusted allowlist is rejected. On success it
// returns the uppercase hex fingerprint of the signer's primary key.
func (k *Keyring) VerifyDetached(signed, sig io.Reader) (string, error) {
	signer, err := openpgp.CheckDetachedSignatureAndHash(k.entities, signed, sig, allowedHashes, nil)
	if err != nil {
		return "", fmt.Errorf("signature verification failed: %w", err)
	}
	if signer == nil || signer.PrimaryKey == nil {
		return "", fmt.Errorf("signature verification produced no signer")
	}

	now := time.Now()
	if selfSig, _ := signer.PrimarySelfSignature(); selfSig == nil || signer.PrimaryKey.KeyExpired(selfSig, now) {
		return "", fmt.Errorf("signing key is expired or has no valid self-signature")
	}
	if signer.Revoked(now) {
		return "", fmt.Errorf("signing key is revoked")
	}

	fingerprint := strings.ToUpper(fmt.Sprintf("%X", signer.PrimaryKey.Fingerprint))
	if len(k.trusted) > 0 && !k.trusted[fingerprint] {
		return "", fmt.Errorf("signing key %s is not in the trusted allowlist", fingerprint)
	}
	return fingerprint, nil
}
