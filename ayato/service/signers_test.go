package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

type memSignerRepo struct{ m map[string][]byte }

func newMemSignerRepo() *memSignerRepo { return &memSignerRepo{m: map[string][]byte{}} }

func (r *memSignerRepo) AddSigner(fpr string, armored []byte) error {
	r.m[fpr] = append([]byte(nil), armored...)
	return nil
}

func (r *memSignerRepo) ListSigners() ([][]byte, error) {
	out := make([][]byte, 0, len(r.m))
	for _, v := range r.m {
		out = append(out, v)
	}
	return out, nil
}

func (r *memSignerRepo) DeleteSigner(fpr string) error {
	delete(r.m, fpr)
	return nil
}

func newServiceWithMaster(t *testing.T, ks *sign.Keystore) (*Service, *memSignerRepo) {
	t.Helper()
	master, err := ks.MasterPublicArmored()
	if err != nil {
		t.Fatal(err)
	}
	repo := newMemSignerRepo()
	cfg := &conf.AyatoConfig{}
	cfg.Verify.MasterKeys = []string{master}
	return New(nil, nil, nil, repo, cfg).(*Service), repo
}

func TestRegisterSignerAcceptsCertifiedWorker(t *testing.T) {
	dir := t.TempDir()
	ks, err := sign.OpenOrCreate(dir, "worker", "w@example.test", "")
	if err != nil {
		t.Fatal(err)
	}
	svc, repo := newServiceWithMaster(t, ks)

	cert, err := ks.WorkerCertArmored()
	if err != nil {
		t.Fatal(err)
	}
	fpr, err := svc.RegisterSigner([]byte(cert))
	if err != nil {
		t.Fatalf("RegisterSigner of a master-certified worker must succeed: %v", err)
	}
	if fpr == "" || len(repo.m) != 1 {
		t.Fatalf("registration should persist one signer, got fpr=%q len=%d", fpr, len(repo.m))
	}

	// A package signed by the registered worker must now verify via the composite keyring.
	pkgPath := filepath.Join(dir, "p-1.0-1-x86_64.pkg.tar.zst")
	if err := os.WriteFile(pkgPath, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	sigPath, err := sign.NewHostKeySigner(ks).Sign(context.Background(), pkgPath)
	if err != nil {
		t.Fatal(err)
	}
	kr, err := svc.verifyKeyring()
	if err != nil || kr == nil {
		t.Fatalf("verifyKeyring must yield a keyring after registration: kr=%v err=%v", kr, err)
	}
	pkg, _ := os.Open(pkgPath)
	defer func() { _ = pkg.Close() }()
	sig, _ := os.Open(sigPath)
	defer func() { _ = sig.Close() }()
	if _, verr := kr.VerifyDetached(pkg, sig); verr != nil {
		t.Fatalf("worker signature must verify against the composite keyring: %v", verr)
	}
}

func TestUnregisterSignerRemovesTrust(t *testing.T) {
	dir := t.TempDir()
	ks, err := sign.OpenOrCreate(dir, "worker", "w@example.test", "")
	if err != nil {
		t.Fatal(err)
	}
	svc, _ := newServiceWithMaster(t, ks)

	cert, _ := ks.WorkerCertArmored()
	fpr, err := svc.RegisterSigner([]byte(cert))
	if err != nil {
		t.Fatal(err)
	}
	if kr, _ := svc.verifyKeyring(); kr == nil {
		t.Fatal("registered worker should yield a keyring")
	}
	if err := svc.UnregisterSigner(fpr); err != nil {
		t.Fatalf("UnregisterSigner: %v", err)
	}
	if kr, _ := svc.verifyKeyring(); kr != nil {
		t.Fatal("revoked worker must no longer be trusted")
	}
}

// A registered (master-certified) worker must verify even when verify.trusted_keys
// pins an unrelated fingerprint, since the master chain already gates it.
func TestRegisteredWorkerBypassesAllowlist(t *testing.T) {
	dir := t.TempDir()
	ks, err := sign.OpenOrCreate(dir, "worker", "w@example.test", "")
	if err != nil {
		t.Fatal(err)
	}
	master, _ := ks.MasterPublicArmored()
	cfg := &conf.AyatoConfig{}
	cfg.Verify.MasterKeys = []string{master}
	cfg.Verify.TrustedKeys = []string{"0000000000000000000000000000000000000000"}
	svc := New(nil, nil, nil, newMemSignerRepo(), cfg).(*Service)

	cert, _ := ks.WorkerCertArmored()
	if _, err := svc.RegisterSigner([]byte(cert)); err != nil {
		t.Fatal(err)
	}

	pkgPath := filepath.Join(dir, "p-1.0-1-x86_64.pkg.tar.zst")
	if err := os.WriteFile(pkgPath, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	sigPath, err := sign.NewHostKeySigner(ks).Sign(context.Background(), pkgPath)
	if err != nil {
		t.Fatal(err)
	}
	kr, err := svc.verifyKeyring()
	if err != nil || kr == nil {
		t.Fatalf("verifyKeyring: kr=%v err=%v", kr, err)
	}
	pkg, _ := os.Open(pkgPath)
	defer func() { _ = pkg.Close() }()
	sig, _ := os.Open(sigPath)
	defer func() { _ = sig.Close() }()
	if _, verr := kr.VerifyDetached(pkg, sig); verr != nil {
		t.Fatalf("registered worker must verify despite the allowlist: %v", verr)
	}
}

func TestRegisterSignerRejectsForeignWorker(t *testing.T) {
	ours, err := sign.OpenOrCreate(t.TempDir(), "ours", "o@example.test", "")
	if err != nil {
		t.Fatal(err)
	}
	svc, _ := newServiceWithMaster(t, ours)

	// A worker certified by a DIFFERENT master must be rejected.
	foreign, err := sign.OpenOrCreate(t.TempDir(), "foreign", "f@example.test", "")
	if err != nil {
		t.Fatal(err)
	}
	cert, err := foreign.WorkerCertArmored()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RegisterSigner([]byte(cert)); err == nil {
		t.Fatal("a worker not certified by the configured master must be rejected")
	}
}
