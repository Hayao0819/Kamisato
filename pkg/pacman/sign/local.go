package sign

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// LoadArmoredEntity parses an armored OpenPGP private key from an in-memory string
// (e.g. an env var) and unlocks it with passphrase when protected. It is the
// env-supplied counterpart to NewLocalSigner's file loader.
func LoadArmoredEntity(armored, passphrase string) (*openpgp.Entity, error) {
	el, err := openpgp.ReadArmoredKeyRing(strings.NewReader(armored))
	if err != nil {
		return nil, fmt.Errorf("parse armored key: %w", err)
	}
	if len(el) != 1 {
		return nil, fmt.Errorf("expected one key, got %d", len(el))
	}
	key := el[0]
	if key.PrivateKey == nil {
		return nil, fmt.Errorf("key has no private material")
	}
	if err := decryptPrivate(key, passphrase); err != nil {
		return nil, err
	}
	return key, nil
}

// LocalSigner signs with an arbitrary OpenPGP private key the user holds locally.
type LocalSigner struct{ key *openpgp.Entity }

// NewLocalSigner loads a private key (armored or binary) from keyPath, decrypting
// it with passphrase when the key is protected.
func NewLocalSigner(keyPath, passphrase string) (*LocalSigner, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read signing key %q: %w", keyPath, err)
	}

	var el openpgp.EntityList
	if bytes.HasPrefix(data, []byte("-----BEGIN PGP")) {
		el, err = openpgp.ReadArmoredKeyRing(bytes.NewReader(data))
	} else {
		el, err = openpgp.ReadKeyRing(bytes.NewReader(data))
	}
	if err != nil {
		return nil, fmt.Errorf("parse signing key %q: %w", keyPath, err)
	}
	if len(el) != 1 {
		return nil, fmt.Errorf("signing key %q: expected one key, got %d", keyPath, len(el))
	}

	key := el[0]
	if key.PrivateKey == nil {
		return nil, fmt.Errorf("signing key %q has no private material", keyPath)
	}
	if err := decryptPrivate(key, passphrase); err != nil {
		return nil, err
	}
	return &LocalSigner{key: key}, nil
}

func (s *LocalSigner) Sign(ctx context.Context, pkgPath string) (string, error) {
	return detachSign(ctx, s.key, pkgPath)
}

func decryptPrivate(e *openpgp.Entity, passphrase string) error {
	pass := []byte(passphrase)
	if e.PrivateKey.Encrypted {
		if err := e.PrivateKey.Decrypt(pass); err != nil {
			return fmt.Errorf("decrypt private key: %w", err)
		}
	}
	for _, sk := range e.Subkeys {
		if sk.PrivateKey != nil && sk.PrivateKey.Encrypted {
			if err := sk.PrivateKey.Decrypt(pass); err != nil {
				return fmt.Errorf("decrypt subkey: %w", err)
			}
		}
	}
	return nil
}
