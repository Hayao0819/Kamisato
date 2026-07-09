package sign

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// ImportSigningKey adopts an existing armored private key into the keystore without minting a new one
// (preserving the trusted primary fingerprint). The key must be a single entity; passphrase unlocks it
// and re-encrypts at rest; existing subkeys are preserved.
func ImportSigningKey(dir string, r io.Reader, passphrase string, force bool) (*SigningKey, error) {
	if _, err := os.Stat(filepath.Join(dir, signingKeyFile)); err == nil && !force {
		return nil, fmt.Errorf("a signing key already exists in %s (pass --force to overwrite)", dir)
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var el openpgp.EntityList
	if bytes.HasPrefix(bytes.TrimLeft(data, " \t\r\n"), []byte("-----BEGIN PGP")) {
		el, err = openpgp.ReadArmoredKeyRing(bytes.NewReader(data))
	} else {
		el, err = openpgp.ReadKeyRing(bytes.NewReader(data))
	}
	if err != nil {
		return nil, fmt.Errorf("parse imported key: %w", err)
	}
	if len(el) != 1 {
		return nil, fmt.Errorf("expected exactly one key, got %d", len(el))
	}
	entity := el[0]
	if entity.PrivateKey == nil {
		return nil, fmt.Errorf("imported data has no private material; export the secret key (gpg --export-secret-keys)")
	}
	if err := decryptPrivate(entity, passphrase); err != nil {
		return nil, fmt.Errorf("decrypt imported key (wrong or missing passphrase?): %w", err)
	}

	k := &SigningKey{dir: dir, entity: entity}
	if err := k.save(passphrase); err != nil {
		return nil, err
	}
	return k, nil
}

// ExpireTargets selects what SetExpiry re-dates.
type ExpireTargets struct {
	Primary    bool   // the primary key's own validity
	AllSubkeys bool   // every non-revoked signing subkey
	Subkey     string // a single subkey by fingerprint (optional)
}

// SetExpiry extends validity to dur from now (0 = never) for the selected parts, re-signing with fresh timestamps.
// Requires the primary secret; OpenPGP measures validity from creation time, so dur is converted per-key to land expiry at now+dur.
func (k *SigningKey) SetExpiry(dur time.Duration, targets ExpireTargets, passphrase string) error {
	if !k.HasPrimarySecret() {
		return errNoPrimarySecret
	}
	now := time.Now()
	changed := false

	if targets.Primary {
		lt := lifetimeFrom(k.entity.PrimaryKey.CreationTime, now, dur)
		for name, ident := range k.entity.Identities {
			ident.SelfSignature.KeyLifetimeSecs = lt
			ident.SelfSignature.CreationTime = now
			if err := ident.SelfSignature.SignUserId(name, k.entity.PrimaryKey, k.entity.PrivateKey, keyConfig()); err != nil {
				return fmt.Errorf("re-sign primary self-signature: %w", err)
			}
		}
		changed = true
	}

	want := NormalizeFingerprint(targets.Subkey)
	for i := range k.entity.Subkeys {
		sk := &k.entity.Subkeys[i]
		selected := (targets.AllSubkeys && sk.Sig != nil && sk.Sig.FlagSign && !sk.Revoked(now)) ||
			(want != "" && Fingerprint(sk.PublicKey.Fingerprint) == want)
		if !selected {
			continue
		}
		lt := lifetimeFrom(sk.PublicKey.CreationTime, now, dur)
		sk.Sig.KeyLifetimeSecs = lt
		sk.Sig.CreationTime = now
		if err := sk.Sig.SignKey(sk.PublicKey, k.entity.PrivateKey, keyConfig()); err != nil {
			return fmt.Errorf("re-sign subkey binding: %w", err)
		}
		changed = true
	}
	if want != "" {
		found := false
		for i := range k.entity.Subkeys {
			if Fingerprint(k.entity.Subkeys[i].PublicKey.Fingerprint) == want {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("no subkey with fingerprint %s", want)
		}
	}
	if !changed {
		return fmt.Errorf("nothing selected to expire (choose primary, all subkeys, or a subkey)")
	}
	return k.save(passphrase)
}

// lifetimeFrom returns KeyLifetimeSecs for a key created at creation to expire at now+dur; dur <= 0 = no expiry.
func lifetimeFrom(creation, now time.Time, dur time.Duration) *uint32 {
	if dur <= 0 {
		var zero uint32
		return &zero
	}
	secs := now.Add(dur).Sub(creation).Seconds()
	if secs < 1 {
		secs = 1
	}
	var lt uint32
	const maxU32 = float64(^uint32(0))
	if secs > maxU32 {
		lt = ^uint32(0)
	} else {
		lt = uint32(secs)
	}
	return &lt
}
