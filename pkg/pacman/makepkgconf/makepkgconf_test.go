package makepkgconf

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func requireBash(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
}

func TestReadFile(t *testing.T) {
	requireBash(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "makepkg.conf")
	// A partial config plus an expansion, to prove bash resolves it (not a parser).
	content := "CARCH=aarch64\n" +
		"CHOST=\"${CARCH}-unknown-linux-gnu\"\n" +
		"PKGDEST=/tmp/pkgs\n" +
		"PKGEXT='.pkg.tar.zst'\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CARCH != "aarch64" {
		t.Errorf("CARCH = %q, want aarch64", cfg.CARCH)
	}
	if cfg.CHOST != "aarch64-unknown-linux-gnu" {
		t.Errorf("CHOST = %q, want aarch64-unknown-linux-gnu (expansion)", cfg.CHOST)
	}
	if cfg.PKGDEST != "/tmp/pkgs" {
		t.Errorf("PKGDEST = %q, want /tmp/pkgs", cfg.PKGDEST)
	}
	if cfg.PKGEXT != ".pkg.tar.zst" {
		t.Errorf("PKGEXT = %q, want .pkg.tar.zst", cfg.PKGEXT)
	}
}

func TestReadFileMissing(t *testing.T) {
	requireBash(t)
	cfg, err := ReadFile(filepath.Join(t.TempDir(), "absent.conf"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if cfg.CARCH != "" || cfg.PKGDEST != "" {
		t.Errorf("missing file should yield empty Conf, got %+v", cfg)
	}
}
