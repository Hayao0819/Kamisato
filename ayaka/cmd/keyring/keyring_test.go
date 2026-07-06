package keyringcmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

func newKeyHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := sign.GenerateSigningKey(dir, "MyRepo", "repo@example.com", 0, 365*24*time.Hour, ""); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return dir
}

func runKeyring(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := Cmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestBuildProducesInstallablePackage(t *testing.T) {
	home := newKeyHome(t)
	out := t.TempDir()

	stdout, err := runKeyring(t, "build",
		"--key-home", home,
		"--name", "myrepo",
		"--version", "20260707-1",
		"--packager", "MyRepo <repo@example.com>",
		"--output-dir", out,
	)
	if err != nil {
		t.Fatalf("build: %v\n%s", err, stdout)
	}

	pkgPath := filepath.Join(out, "myrepo-keyring-20260707-1-any.pkg.tar.zst")
	if !strings.Contains(stdout, pkgPath) {
		t.Errorf("build should print the package path, got:\n%s", stdout)
	}
	meta, err := pkg.ReadBinaryPackageMeta(pkgPath)
	if err != nil {
		t.Fatalf("built package is not readable: %v", err)
	}
	if meta.Info.PkgName != "myrepo-keyring" {
		t.Errorf("pkgname = %q", meta.Info.PkgName)
	}
}

func TestBuildRequiresName(t *testing.T) {
	home := newKeyHome(t)
	if _, err := runKeyring(t, "build", "--key-home", home, "--output-dir", t.TempDir()); err == nil {
		t.Error("build without --name should fail")
	}
}

func TestBootstrapPrintsFingerprint(t *testing.T) {
	home := newKeyHome(t)
	k, err := sign.LoadSigningKey(home, "")
	if err != nil {
		t.Fatal(err)
	}
	out, err := runKeyring(t, "bootstrap", "--key-home", home, "--repo", "myrepo", "--base-url", "https://repo.example")
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if !strings.Contains(out, k.PrimaryFingerprint()) {
		t.Errorf("bootstrap should embed the fingerprint:\n%s", out)
	}
	if !strings.Contains(out, "pacman-key --lsign-key") || !strings.Contains(out, "Server = https://repo.example/$arch") {
		t.Errorf("bootstrap missing expected steps:\n%s", out)
	}
}
