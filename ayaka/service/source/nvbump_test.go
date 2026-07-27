package source

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

func TestRewritePkgver(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"plain", "pkgname=foo\npkgver=1.0\npkgrel=3\narch=(any)\n", "pkgname=foo\npkgver=2.0\npkgrel=1\narch=(any)\n"},
		{"crlf", "pkgver=1.0\r\npkgrel=2\r\n", "pkgver=2.0\r\npkgrel=1\r\n"},
		{"quoted", "pkgver='1.0'\npkgrel='4'\n", "pkgver='2.0'\npkgrel='1'\n"},
		{"rebuild suffix reset", "pkgver=1.0\npkgrel=1.2\n", "pkgver=2.0\npkgrel=1\n"},
		{"derived vars untouched", "pkgver=1.0\n_pkgver=${pkgver%.*}\npkgrel=1\n", "pkgver=2.0\n_pkgver=${pkgver%.*}\npkgrel=1\n"},
	}
	for _, tt := range tests {
		out, err := rewritePkgver([]byte(tt.in), "2.0")
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if string(out) != tt.want {
			t.Errorf("%s: rewritePkgver = %q, want %q", tt.name, out, tt.want)
		}
	}

	if _, err := rewritePkgver([]byte("pkgrel=1\n"), "2.0"); err == nil {
		t.Error("rewritePkgver without pkgver should error")
	}
	if _, err := rewritePkgver([]byte("pkgver=1.0\n"), "2.0"); err == nil {
		t.Error("rewritePkgver without pkgrel should error")
	}
}

func TestNvBump(t *testing.T) {
	for _, bin := range []string{"makepkg", "updpkgsums"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not on PATH", bin)
		}
	}

	dir := t.TempDir()
	pkgbuild := "pkgname=foo\npkgver=1.0\npkgrel=3\npkgdesc='test'\narch=(any)\nsource=(foo.txt)\nsha256sums=('SKIP')\n"
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(pkgbuild), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foo.txt"), []byte("payload\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := repo.GenerateSrcinfo(dir, os.Stderr); err != nil {
		t.Fatal(err)
	}
	p, err := pkg.OpenSourcePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	src := &repo.SourceRepo{Config: &repo.SrcConfig{Name: "test"}, Pkgs: []*pkg.SourcePackage{p}, Dir: dir}

	bumped, err := NvBump(src, "foo", "2.0", os.Stderr)
	if err != nil {
		t.Fatal(err)
	}
	if bumped.Version() != "2.0-1" {
		t.Fatalf("bumped version = %q, want 2.0-1", bumped.Version())
	}
	data, err := os.ReadFile(filepath.Join(dir, "PKGBUILD"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "pkgver=2.0") || !strings.Contains(string(data), "pkgrel=1") {
		t.Errorf("PKGBUILD not rewritten: %q", data)
	}
	if strings.Contains(string(data), "SKIP") {
		t.Errorf("updpkgsums should replace the checksum: %q", data)
	}

	if _, err := NvBump(src, "nope", "2.0", os.Stderr); err == nil {
		t.Error("bumping an unknown package should error")
	}
}
