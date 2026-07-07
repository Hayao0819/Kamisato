package sign

import (
	"crypto"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

const (
	masterPubFile  = "master.pub"
	workerKeyFile  = "worker.key"
	workerCertFile = "worker.pub"
)

// keyConfig pins Ed25519 keys and SHA-256 digests so signatures land inside the
// hash set ayato's verifier accepts.
func keyConfig() *packet.Config {
	return &packet.Config{Algorithm: packet.PubKeyAlgoEdDSA, DefaultHash: crypto.SHA256}
}

// Keystore is a worker signing key (private) plus the certifying master (public); the master is the trust root ayato pins.
type Keystore struct {
	dir    string
	master *openpgp.Entity // public only
	worker *openpgp.Entity // private
}

// OpenOrCreate loads the keystore in dir, or generates a certified master+worker pair on first run. Passphrase encrypts the worker key at rest.
func OpenOrCreate(dir, name, email, passphrase string) (*Keystore, error) {
	if _, err := os.Stat(filepath.Join(dir, workerKeyFile)); err == nil {
		return load(dir, passphrase)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return create(dir, name, email, passphrase)
}

func create(dir, name, email, passphrase string) (*Keystore, error) {
	cfg := keyConfig()
	master, err := openpgp.NewEntity(name+" master", "kamisato signing master", email, cfg)
	if err != nil {
		return nil, fmt.Errorf("generate master key: %w", err)
	}
	worker, err := openpgp.NewEntity(name, "kamisato signing worker", email, cfg)
	if err != nil {
		return nil, fmt.Errorf("generate worker key: %w", err)
	}
	for uid := range worker.Identities {
		if err := worker.SignIdentity(uid, master, cfg); err != nil {
			return nil, fmt.Errorf("master certify worker: %w", err)
		}
	}
	k := &Keystore{dir: dir, master: master, worker: worker}
	if err := k.save(passphrase); err != nil {
		return nil, err
	}
	return k, nil
}

func (k *Keystore) save(passphrase string) error {
	if err := os.MkdirAll(k.dir, 0o700); err != nil {
		return err
	}
	if err := writeArmored(filepath.Join(k.dir, masterPubFile), openpgp.PublicKeyType, 0o644, k.master.Serialize); err != nil {
		return fmt.Errorf("write master pub: %w", err)
	}
	if err := writeArmored(filepath.Join(k.dir, workerCertFile), openpgp.PublicKeyType, 0o644, k.worker.Serialize); err != nil {
		return fmt.Errorf("write worker cert: %w", err)
	}

	// Encrypt the private key for disk, then restore the in-memory copy so this
	// keystore can still sign in the same process.
	if passphrase != "" {
		if err := k.worker.EncryptPrivateKeys([]byte(passphrase), &packet.Config{}); err != nil {
			return fmt.Errorf("encrypt worker key: %w", err)
		}
	}
	keyPath := filepath.Join(k.dir, workerKeyFile)
	if err := writeArmored(keyPath, openpgp.PrivateKeyType, 0o600, func(w io.Writer) error {
		return k.worker.SerializePrivateWithoutSigning(w, nil)
	}); err != nil {
		return fmt.Errorf("write worker key: %w", err)
	}
	// OpenFile honors the umask, so force 0600 on the private key explicitly.
	if err := os.Chmod(keyPath, 0o600); err != nil {
		return err
	}
	if passphrase != "" {
		return decryptPrivate(k.worker, passphrase)
	}
	return nil
}

func load(dir, passphrase string) (*Keystore, error) {
	master, err := readEntity(filepath.Join(dir, masterPubFile))
	if err != nil {
		return nil, fmt.Errorf("load master pub: %w", err)
	}
	worker, err := readEntity(filepath.Join(dir, workerKeyFile))
	if err != nil {
		return nil, fmt.Errorf("load worker key: %w", err)
	}
	if worker.PrivateKey != nil && worker.PrivateKey.Encrypted {
		if err := decryptPrivate(worker, passphrase); err != nil {
			return nil, fmt.Errorf("decrypt worker key (wrong or missing MIKO_SIGNING_PASSPHRASE?): %w", err)
		}
	}
	return &Keystore{dir: dir, master: master, worker: worker}, nil
}

// WorkerEntity is the private signing key HostKeySigner uses.
func (k *Keystore) WorkerEntity() *openpgp.Entity { return k.worker }

// MasterEntity is the public master that ayato pins as its trust root.
func (k *Keystore) MasterEntity() *openpgp.Entity { return k.master }

// MasterPublicArmored returns the master public key for ayato's verify.master_keys.
func (k *Keystore) MasterPublicArmored() (string, error) {
	return readString(filepath.Join(k.dir, masterPubFile))
}

// WorkerCertArmored returns the master-certified worker public key for registration.
func (k *Keystore) WorkerCertArmored() (string, error) {
	return readString(filepath.Join(k.dir, workerCertFile))
}

// CertifiedBy returns nil if a UID of child carries a valid certification by parent's primary key (the worker←master chain ayato enforces).
func CertifiedBy(child, parent *openpgp.Entity) error {
	for name, ident := range child.Identities {
		for _, sig := range ident.Signatures {
			if sig.IssuerKeyId == nil || *sig.IssuerKeyId != parent.PrimaryKey.KeyId {
				continue
			}
			if err := parent.PrimaryKey.VerifyUserIdSignature(name, child.PrimaryKey, sig); err == nil {
				return nil
			}
		}
	}
	return fmt.Errorf("no valid certification by master %X", parent.PrimaryKey.KeyId)
}

func readEntity(path string) (*openpgp.Entity, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	el, err := openpgp.ReadArmoredKeyRing(f)
	if err != nil {
		return nil, err
	}
	if len(el) != 1 {
		return nil, fmt.Errorf("%s: expected one key, got %d", path, len(el))
	}
	return el[0], nil
}

// writeArmored writes armored data to path atomically (temp file → fsync → rename) so a crash never truncates the key.
// os.CreateTemp sets 0600 so the private key is never briefly world-readable.
func writeArmored(path, blockType string, perm os.FileMode, serialize func(io.Writer) error) (err error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// On any failure, drop the temp file so we do not litter the key directory.
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	a, aerr := armor.Encode(tmp, blockType, nil)
	if aerr != nil {
		return aerr
	}
	if serr := serialize(a); serr != nil {
		_ = a.Close()
		return serr
	}
	if cerr := a.Close(); cerr != nil {
		return cerr
	}
	if cherr := tmp.Chmod(perm); cherr != nil {
		return cherr
	}
	if serr := tmp.Sync(); serr != nil {
		return serr
	}
	if cerr := tmp.Close(); cerr != nil {
		return cerr
	}
	if rerr := os.Rename(tmpName, path); rerr != nil {
		return rerr
	}
	return fsyncDir(dir)
}

// fsyncDir flushes the directory so the rename is durable; failure to open is non-fatal on platforms that disallow dir fsync.
func fsyncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return nil
	}
	defer func() { _ = d.Close() }()
	_ = d.Sync()
	return nil
}

func readString(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
