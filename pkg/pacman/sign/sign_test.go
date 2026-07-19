package sign

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
)

func writePkg(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatalf("write pkg: %v", err)
	}
	return p
}

func verify(t *testing.T, pubPath, pkgPath, sigPath string) (string, error) {
	t.Helper()
	kr, err := LoadKeyring(pubPath, nil)
	if err != nil {
		t.Fatalf("load keyring: %v", err)
	}
	pkg, err := os.Open(pkgPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pkg.Close() }()
	sig, err := os.Open(sigPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sig.Close() }()
	return kr.VerifyDetached(pkg, sig)
}

func TestHostKeySignerRoundTrip(t *testing.T) {
	dir := t.TempDir()
	k, err := OpenOrCreate(dir, "miko worker", "worker@example.test", "")
	if err != nil {
		t.Fatalf("OpenOrCreate: %v", err)
	}

	pkgPath := writePkg(t, dir, "foo-1.0-1-x86_64.pkg.tar.zst", []byte("package bytes"))
	sigPath, err := NewHostKeySigner(k).Sign(context.Background(), pkgPath)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	pubPath := filepath.Join(dir, workerCertFile)
	if _, err := verify(t, pubPath, pkgPath, sigPath); err != nil {
		t.Fatalf("worker signature must verify against the worker cert: %v", err)
	}

	// A tampered package must fail verification.
	if err := os.WriteFile(pkgPath, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := verify(t, pubPath, pkgPath, sigPath); err == nil {
		t.Fatal("tampered package must not verify")
	}
}

func TestWorkerCertifiedByMaster(t *testing.T) {
	dir := t.TempDir()
	k, err := OpenOrCreate(dir, "miko worker", "worker@example.test", "")
	if err != nil {
		t.Fatalf("OpenOrCreate: %v", err)
	}

	cert, err := readEntity(filepath.Join(dir, workerCertFile))
	if err != nil {
		t.Fatalf("read worker cert: %v", err)
	}
	if err := CertifiedBy(cert, k.MasterEntity()); err != nil {
		t.Fatalf("worker must be certified by its master: %v", err)
	}

	other, err := openpgp.NewEntity("other master", "", "other@example.test", keyConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := CertifiedBy(cert, other); err == nil {
		t.Fatal("worker must not appear certified by an unrelated master")
	}
}

func TestOpenOrCreateIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	first, err := OpenOrCreate(dir, "miko worker", "worker@example.test", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := OpenOrCreate(dir, "miko worker", "worker@example.test", "")
	if err != nil {
		t.Fatal(err)
	}
	if first.worker.PrimaryKey.KeyId != second.worker.PrimaryKey.KeyId {
		t.Fatal("reopening the keystore must not regenerate the worker key")
	}
}

func TestOpenOrCreateSerializesConcurrentInitialization(t *testing.T) {
	dir := t.TempDir()
	const callers = 4
	start := make(chan struct{})
	results := make(chan *Keystore, callers)
	errs := make(chan error, callers)

	for range callers {
		go func() {
			<-start
			k, err := OpenOrCreate(dir, "miko worker", "worker@example.test", "")
			if err != nil {
				errs <- err
				return
			}
			results <- k
		}()
	}
	close(start)

	var first *Keystore
	for range callers {
		select {
		case err := <-errs:
			t.Fatalf("OpenOrCreate: %v", err)
		case got := <-results:
			if err := CertifiedBy(got.worker, got.master); err != nil {
				t.Fatalf("concurrent caller loaded inconsistent keys: %v", err)
			}
			if first == nil {
				first = got
				continue
			}
			if got.master.PrimaryKey.KeyId != first.master.PrimaryKey.KeyId ||
				got.worker.PrimaryKey.KeyId != first.worker.PrimaryKey.KeyId {
				t.Fatal("concurrent initialization returned different key pairs")
			}
		}
	}
}

func TestEncryptedKeystore(t *testing.T) {
	dir := t.TempDir()
	if _, err := OpenOrCreate(dir, "worker", "worker@example.test", "s3cret"); err != nil {
		t.Fatalf("create encrypted: %v", err)
	}

	// Reload with the right passphrase and sign.
	k, err := OpenOrCreate(dir, "worker", "worker@example.test", "s3cret")
	if err != nil {
		t.Fatalf("reload encrypted: %v", err)
	}
	pkgPath := writePkg(t, dir, "enc-1.0-1-x86_64.pkg.tar.zst", []byte("payload"))
	sigPath, err := NewHostKeySigner(k).Sign(context.Background(), pkgPath)
	if err != nil {
		t.Fatalf("sign after reload: %v", err)
	}
	if _, err := verify(t, filepath.Join(dir, workerCertFile), pkgPath, sigPath); err != nil {
		t.Fatalf("signature must verify after encrypted reload: %v", err)
	}

	// A wrong or missing passphrase must fail to load the encrypted key.
	if _, err := OpenOrCreate(dir, "worker", "worker@example.test", "wrong"); err == nil {
		t.Fatal("wrong passphrase must not load")
	}
	if _, err := OpenOrCreate(dir, "worker", "worker@example.test", ""); err == nil {
		t.Fatal("missing passphrase must not load an encrypted key")
	}
}

func TestLocalSigner(t *testing.T) {
	dir := t.TempDir()
	k, err := OpenOrCreate(dir, "dev", "dev@example.test", "")
	if err != nil {
		t.Fatal(err)
	}

	// The worker private key doubles as an arbitrary local key for this test.
	s, err := NewLocalSigner(filepath.Join(dir, workerKeyFile), "")
	if err != nil {
		t.Fatalf("NewLocalSigner: %v", err)
	}
	pkgPath := writePkg(t, dir, "bar-1.0-1-x86_64.pkg.tar.zst", []byte("local pkg"))
	sigPath, err := s.Sign(context.Background(), pkgPath)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if _, err := verify(t, filepath.Join(dir, workerCertFile), pkgPath, sigPath); err != nil {
		t.Fatalf("locally-signed package must verify: %v", err)
	}
	_ = k
}
