package repo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateSrcinfo(t *testing.T) {
	if _, err := exec.LookPath("makepkg"); err != nil {
		t.Skip("makepkg not available")
	}

	dir := t.TempDir()
	pkgbuild := "pkgname=foo\npkgver=1.0\npkgrel=1\narch=('any')\n"
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(pkgbuild), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := GenerateSrcinfo(dir, os.Stderr); err != nil {
		t.Fatalf("GenerateSrcinfo: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, ".SRCINFO"))
	if err != nil {
		t.Fatalf("read .SRCINFO: %v", err)
	}
	if !strings.Contains(string(got), "pkgver = 1.0") {
		t.Fatalf(".SRCINFO missing regenerated pkgver:\n%s", got)
	}
}

// A failing PKGBUILD must not clobber an existing .SRCINFO.
func TestGenerateSrcinfo_KeepsOldOnFailure(t *testing.T) {
	if _, err := exec.LookPath("makepkg"); err != nil {
		t.Skip("makepkg not available")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte("this is not valid bash ((("), 0o644); err != nil {
		t.Fatal(err)
	}
	old := "pkgbase = foo\n"
	srcinfoPath := filepath.Join(dir, ".SRCINFO")
	if err := os.WriteFile(srcinfoPath, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := GenerateSrcinfo(dir, os.Stderr); err == nil {
		t.Fatal("GenerateSrcinfo: want error for invalid PKGBUILD, got nil")
	}

	got, err := os.ReadFile(srcinfoPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != old {
		t.Fatalf("existing .SRCINFO was clobbered: %q", got)
	}
}
