package sign

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

// Keyring is the trust root for detached package signature verification;
// trusted is an optional fingerprint allowlist (empty means accept any key present).
type Keyring struct {
	entities openpgp.EntityList
	trusted  map[string]bool
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
		if n := NormalizeFingerprint(fpr); n != "" {
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

// allowedHashes pins accepted digest algorithms; restricting to SHA-256/384/512 rejects SHA-1 downgrade
// before the signer resolves, closing the algorithm-substitution attack.
var allowedHashes = []crypto.Hash{crypto.SHA256, crypto.SHA384, crypto.SHA512}

// VerifyDetached verifies a detached signature against signed, failing closed (SHA-1, expired/revoked key,
// or untrusted fingerprint all rejected); returns the signer's uppercase primary-key fingerprint.
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
