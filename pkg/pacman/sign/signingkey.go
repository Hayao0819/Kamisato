package sign

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// signingKeyFile is the armored private key held in a SigningKey directory.
const signingKeyFile = "signing.key"

// SigningKey is the repository's own OpenPGP identity: a primary key plus a
// signing subkey, held together in one keyring directory. The primary
// fingerprint is the trust anchor a keyring package pins, so rotating the signing
// subkey never changes what downstream users trust. Package signing goes through
// the subkey; the primary certifies it and issues revocations.
type SigningKey struct {
	dir    string
	entity *openpgp.Entity
}

// GenerateSigningKey creates a fresh primary + signing subkey in dir. primaryTTL
// and subkeyTTL are the validity periods (0 = no expiry). A non-empty passphrase
// encrypts the private key at rest. The encryption subkey go-crypto adds by
// default is dropped: a repo key only ever signs.
func GenerateSigningKey(dir, name, email string, primaryTTL, subkeyTTL time.Duration, passphrase string) (*SigningKey, error) {
	if _, err := os.Stat(filepath.Join(dir, signingKeyFile)); err == nil {
		return nil, fmt.Errorf("a signing key already exists in %s", dir)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	cfg := keyConfig()
	cfg.KeyLifetimeSecs = ttlSecs(primaryTTL)
	entity, err := openpgp.NewEntity(name, "kamisato repository signing key", email, cfg)
	if err != nil {
		return nil, fmt.Errorf("generate primary key: %w", err)
	}

	subCfg := keyConfig()
	subCfg.KeyLifetimeSecs = ttlSecs(subkeyTTL)
	if err := entity.AddSigningSubkey(subCfg); err != nil {
		return nil, fmt.Errorf("add signing subkey: %w", err)
	}
	dropEncryptionSubkeys(entity)

	k := &SigningKey{dir: dir, entity: entity}
	if err := k.save(passphrase); err != nil {
		return nil, err
	}
	return k, nil
}

// LoadSigningKey reads the key from dir, unlocking the private material with
// passphrase when it is encrypted.
func LoadSigningKey(dir, passphrase string) (*SigningKey, error) {
	entity, err := readEntity(filepath.Join(dir, signingKeyFile))
	if err != nil {
		return nil, fmt.Errorf("load signing key: %w", err)
	}
	if entity.PrivateKey == nil {
		return nil, fmt.Errorf("signing key in %s has no private material", dir)
	}
	if err := decryptPrivate(entity, passphrase); err != nil {
		return nil, fmt.Errorf("decrypt signing key (wrong or missing passphrase?): %w", err)
	}
	return &SigningKey{dir: dir, entity: entity}, nil
}

func (k *SigningKey) save(passphrase string) error {
	if err := os.MkdirAll(k.dir, 0o700); err != nil {
		return err
	}
	if passphrase != "" {
		if err := k.entity.EncryptPrivateKeys([]byte(passphrase), &packet.Config{}); err != nil {
			return fmt.Errorf("encrypt signing key: %w", err)
		}
	}
	keyPath := filepath.Join(k.dir, signingKeyFile)
	if err := writeArmored(keyPath, openpgp.PrivateKeyType, 0o600, func(w io.Writer) error {
		return k.entity.SerializePrivateWithoutSigning(w, nil)
	}); err != nil {
		return fmt.Errorf("write signing key: %w", err)
	}
	if err := os.Chmod(keyPath, 0o600); err != nil {
		return err
	}
	if passphrase != "" {
		return decryptPrivate(k.entity, passphrase)
	}
	return nil
}

// Entity exposes the underlying key (private material included) for signing and
// inspection.
func (k *SigningKey) Entity() *openpgp.Entity { return k.entity }

// PublicEntity returns a public-only copy suitable for a distributable keyring.
func (k *SigningKey) PublicEntity() (*openpgp.Entity, error) {
	var buf bytes.Buffer
	if err := k.entity.Serialize(&buf); err != nil {
		return nil, err
	}
	el, err := openpgp.ReadKeyRing(&buf)
	if err != nil {
		return nil, err
	}
	if len(el) != 1 {
		return nil, fmt.Errorf("expected one public entity, got %d", len(el))
	}
	return el[0], nil
}

// ExportPublicArmored returns the armored public key (primary + subkeys +
// revocation signatures), the material a keyring `.gpg` bundles.
func (k *SigningKey) ExportPublicArmored() (string, error) {
	var buf bytes.Buffer
	a, err := armor.Encode(&buf, openpgp.PublicKeyType, nil)
	if err != nil {
		return "", err
	}
	if err := k.entity.Serialize(a); err != nil {
		_ = a.Close()
		return "", err
	}
	if err := a.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ExportSecretArmored returns the armored private key for offline backup. Handle
// the result as a secret: it is the full key, primary included.
func (k *SigningKey) ExportSecretArmored() (string, error) {
	var buf bytes.Buffer
	a, err := armor.Encode(&buf, openpgp.PrivateKeyType, nil)
	if err != nil {
		return "", err
	}
	if err := k.entity.SerializePrivateWithoutSigning(a, nil); err != nil {
		_ = a.Close()
		return "", err
	}
	if err := a.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// PrimaryFingerprint is the trust anchor: the 40-hex uppercase fingerprint a
// keyring package writes into its `-trusted` file.
func (k *SigningKey) PrimaryFingerprint() string {
	return Fingerprint(k.entity.PrimaryKey.Fingerprint)
}

// HasPrimarySecret reports whether the primary private key is present, which the
// commands needing to add/revoke/rotate require (vs. a subkeys-only working copy).
func (k *SigningKey) HasPrimarySecret() bool {
	return k.entity.PrivateKey != nil && !k.entity.PrivateKey.Dummy()
}

// SubkeyInfo is a signing subkey's state for `key list`.
type SubkeyInfo struct {
	Fingerprint string
	Created     time.Time
	Expires     time.Time // zero means no expiry
	Revoked     bool
	CanSign     bool
}

// Subkeys lists the key's subkeys, newest binding first is not guaranteed; the
// order follows the on-disk packet order.
func (k *SigningKey) Subkeys() []SubkeyInfo {
	now := time.Now()
	out := make([]SubkeyInfo, 0, len(k.entity.Subkeys))
	for i := range k.entity.Subkeys {
		sk := &k.entity.Subkeys[i]
		info := SubkeyInfo{
			Fingerprint: Fingerprint(sk.PublicKey.Fingerprint),
			Created:     sk.PublicKey.CreationTime,
			Revoked:     sk.Revoked(now),
			CanSign:     sk.Sig != nil && sk.Sig.FlagsValid && sk.Sig.FlagSign,
		}
		if sk.Sig != nil && sk.Sig.KeyLifetimeSecs != nil && *sk.Sig.KeyLifetimeSecs != 0 {
			info.Expires = sk.PublicKey.CreationTime.Add(time.Duration(*sk.Sig.KeyLifetimeSecs) * time.Second)
		}
		out = append(out, info)
	}
	return out
}

// Revoked reports whether the primary key itself has been revoked.
func (k *SigningKey) Revoked() bool { return k.entity.Revoked(time.Now()) }

// AddSubkey binds a new signing subkey with the given validity. Requires the
// primary secret.
func (k *SigningKey) AddSubkey(subkeyTTL time.Duration, passphrase string) error {
	if !k.HasPrimarySecret() {
		return errNoPrimarySecret
	}
	cfg := keyConfig()
	cfg.KeyLifetimeSecs = ttlSecs(subkeyTTL)
	if err := k.entity.AddSigningSubkey(cfg); err != nil {
		return fmt.Errorf("add signing subkey: %w", err)
	}
	dropEncryptionSubkeys(k.entity)
	return k.save(passphrase)
}

// RevokeSubkey revokes the subkey with the given fingerprint, recording reason.
// Requires the primary secret.
func (k *SigningKey) RevokeSubkey(fingerprint string, reason packet.ReasonForRevocation, reasonText, passphrase string) error {
	if !k.HasPrimarySecret() {
		return errNoPrimarySecret
	}
	want := normalizeFingerprint(fingerprint)
	for i := range k.entity.Subkeys {
		sk := &k.entity.Subkeys[i]
		if Fingerprint(sk.PublicKey.Fingerprint) != want {
			continue
		}
		if err := k.entity.RevokeSubkey(sk, reason, reasonText, keyConfig()); err != nil {
			return fmt.Errorf("revoke subkey: %w", err)
		}
		return k.save(passphrase)
	}
	return fmt.Errorf("no subkey with fingerprint %s", want)
}

// RotateSubkey revokes every currently-valid signing subkey with reason, then
// binds a fresh one. This is the routine-rotation entry point; use a soft reason
// (superseded) so packages signed by the old subkey stay valid. Requires the
// primary secret.
func (k *SigningKey) RotateSubkey(reason packet.ReasonForRevocation, reasonText string, subkeyTTL time.Duration, passphrase string) error {
	if !k.HasPrimarySecret() {
		return errNoPrimarySecret
	}
	now := time.Now()
	for i := range k.entity.Subkeys {
		sk := &k.entity.Subkeys[i]
		if sk.Sig == nil || !sk.Sig.FlagSign || sk.Revoked(now) {
			continue
		}
		if err := k.entity.RevokeSubkey(sk, reason, reasonText, keyConfig()); err != nil {
			return fmt.Errorf("revoke old subkey: %w", err)
		}
	}
	cfg := keyConfig()
	cfg.KeyLifetimeSecs = ttlSecs(subkeyTTL)
	if err := k.entity.AddSigningSubkey(cfg); err != nil {
		return fmt.Errorf("add new subkey: %w", err)
	}
	dropEncryptionSubkeys(k.entity)
	return k.save(passphrase)
}

// RevokePrimary revokes the whole key, and by extension every subkey. Requires
// the primary secret. Use reason compromised (hard) for a leak.
func (k *SigningKey) RevokePrimary(reason packet.ReasonForRevocation, reasonText, passphrase string) error {
	if !k.HasPrimarySecret() {
		return errNoPrimarySecret
	}
	if err := k.entity.RevokeKey(reason, reasonText, keyConfig()); err != nil {
		return fmt.Errorf("revoke primary key: %w", err)
	}
	return k.save(passphrase)
}

// Sign makes SigningKey a Signer: go-crypto selects the signing subkey.
func (k *SigningKey) Sign(ctx context.Context, pkgPath string) (string, error) {
	return detachSign(ctx, k.entity, pkgPath)
}

var errNoPrimarySecret = fmt.Errorf("this operation needs the primary secret key; run it against the offline key directory with --home")

// dropEncryptionSubkeys removes any non-signing subkey. A repository key only
// signs; the encryption subkey NewEntity always adds is dead weight in a keyring.
func dropEncryptionSubkeys(e *openpgp.Entity) {
	kept := e.Subkeys[:0]
	for _, sk := range e.Subkeys {
		if sk.Sig != nil && sk.Sig.FlagSign {
			kept = append(kept, sk)
		}
	}
	e.Subkeys = kept
}

func ttlSecs(d time.Duration) uint32 {
	if d <= 0 {
		return 0
	}
	secs := d / time.Second
	const maxU32 = uint32(0xffffffff)
	if secs > time.Duration(maxU32) {
		return maxU32
	}
	return uint32(secs)
}
