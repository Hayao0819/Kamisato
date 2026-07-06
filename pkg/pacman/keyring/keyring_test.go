package keyring

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"

	pkgpkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

func newKey(t *testing.T) *sign.SigningKey {
	t.Helper()
	k, err := sign.GenerateSigningKey(t.TempDir(), "myrepo", "repo@example.com", 0, 365*24*time.Hour, "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return k
}

func TestBuildFilesRoundTrip(t *testing.T) {
	k := newKey(t)
	pub, err := k.PublicEntity()
	if err != nil {
		t.Fatal(err)
	}
	fpr := k.PrimaryFingerprint()

	files, err := BuildFiles("myrepo", []*openpgp.Entity{pub}, []string{fpr}, []string{"DEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEF"})
	if err != nil {
		t.Fatalf("build files: %v", err)
	}

	// .gpg must parse back into the same key.
	el, err := openpgp.ReadKeyRing(bytes.NewReader(files.GPG))
	if err != nil {
		t.Fatalf("re-read .gpg: %v", err)
	}
	if len(el) != 1 || sign.Fingerprint(el[0].PrimaryKey.Fingerprint) != fpr {
		t.Errorf(".gpg did not round-trip the key")
	}

	// -trusted lists the anchor fingerprint with :4:.
	if got := string(files.Trusted); got != fpr+":4:\n" {
		t.Errorf("trusted = %q, want %q", got, fpr+":4:\n")
	}
	// -revoked lists the bare fingerprint.
	if got := string(files.Revoked); got != "DEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEF\n" {
		t.Errorf("revoked = %q", got)
	}
}

func TestBuildFilesRejectsEmpty(t *testing.T) {
	if _, err := BuildFiles("", nil, nil, nil); err == nil {
		t.Error("empty name should error")
	}
	if _, err := BuildFiles("x", nil, nil, nil); err == nil {
		t.Error("no keys should error")
	}
}

// TestBuildPackageReadableByPacmanReader builds a keyring package and reads it
// back with the repo's own package parser, proving the tarball layout and
// .PKGINFO satisfy what pacman/repo-add consume.
func TestBuildPackageReadableByPacmanReader(t *testing.T) {
	k := newKey(t)
	pub, err := k.PublicEntity()
	if err != nil {
		t.Fatal(err)
	}
	files, err := BuildFiles("myrepo", []*openpgp.Entity{pub}, []string{k.PrimaryFingerprint()}, nil)
	if err != nil {
		t.Fatal(err)
	}

	data, err := BuildPackage(PackageOpts{
		Files:     files,
		Version:   "20260707-1",
		Packager:  "MyRepo <repo@example.com>",
		Depends:   []string{"archlinux-keyring"},
		BuildDate: time.Unix(1751846400, 0),
	})
	if err != nil {
		t.Fatalf("build package: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, FileName("myrepo", "20260707-1"))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	meta, err := pkgpkg.ReadBinaryPackageMeta(path)
	if err != nil {
		t.Fatalf("repo package reader rejected the keyring package: %v", err)
	}
	if meta.Info.PkgName != "myrepo-keyring" {
		t.Errorf("pkgname = %q, want myrepo-keyring", meta.Info.PkgName)
	}
	if meta.Info.Arch != "any" {
		t.Errorf("arch = %q, want any", meta.Info.Arch)
	}
	if meta.Info.PkgVer != "20260707-1" {
		t.Errorf("pkgver = %q", meta.Info.PkgVer)
	}
	if meta.Info.BuildDate != 1751846400 {
		t.Errorf("builddate = %d", meta.Info.BuildDate)
	}
	if meta.Info.Size <= 0 {
		t.Errorf("installed size should be positive, got %d", meta.Info.Size)
	}

	// The three keyring files must appear in the payload (dotfiles like .PKGINFO
	// are excluded from the files list by the reader).
	want := map[string]bool{
		"usr/share/pacman/keyrings/myrepo.gpg":     false,
		"usr/share/pacman/keyrings/myrepo-trusted": false,
		"usr/share/pacman/keyrings/myrepo-revoked": false,
	}
	for _, f := range meta.Files {
		if _, ok := want[f]; ok {
			want[f] = true
		}
	}
	for f, seen := range want {
		if !seen {
			t.Errorf("payload missing %s (files: %v)", f, meta.Files)
		}
	}
}

// TestPackageSignatureVerifies signs the built package with the same key and
// checks the detached signature against the key that the keyring itself ships —
// the bootstrap guarantee (the keyring package is verifiable by its own key).
func TestPackageSignatureVerifies(t *testing.T) {
	k := newKey(t)
	pub, err := k.PublicEntity()
	if err != nil {
		t.Fatal(err)
	}
	files, err := BuildFiles("myrepo", []*openpgp.Entity{pub}, []string{k.PrimaryFingerprint()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := BuildPackage(PackageOpts{Files: files, Version: "20260707-1", Packager: "x <x@x>"})
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "pkg.tar.zst")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	sigPath, err := k.Sign(context.Background(), path)
	if err != nil {
		t.Fatalf("sign package: %v", err)
	}

	pkgf, _ := os.Open(path)
	defer func() { _ = pkgf.Close() }()
	sigf, _ := os.Open(sigPath)
	defer func() { _ = sigf.Close() }()
	kr, err := openpgp.ReadKeyRing(bytes.NewReader(files.GPG))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := openpgp.CheckDetachedSignature(kr, pkgf, sigf, nil); err != nil {
		t.Fatalf("keyring package not verifiable by its own shipped key: %v", err)
	}
}

func TestFileNameAndInstallScript(t *testing.T) {
	if got := FileName("myrepo", "20260707-1"); got != "myrepo-keyring-20260707-1-any.pkg.tar.zst" {
		t.Errorf("filename = %q", got)
	}
	s := installScript("myrepo")
	if !strings.Contains(s, "pacman-key --populate myrepo") {
		t.Errorf("install script missing populate call:\n%s", s)
	}
}
