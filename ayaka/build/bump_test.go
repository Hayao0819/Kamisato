package build

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

func TestNextPkgrel(t *testing.T) {
	tests := []struct {
		cur, by, want string
		wantErr       bool
	}{
		{cur: "1", by: "0.1", want: "1.1"},
		{cur: "1.1", by: "0.1", want: "1.2"},
		{cur: "1.9", by: "0.1", want: "1.10"},
		{cur: "1", by: "1", want: "2"},
		{cur: "1.2", by: "1", want: "2"},
		{cur: "x", by: "0.1", wantErr: true},
		{cur: "1", by: "2", wantErr: true},
	}
	for _, tt := range tests {
		got, err := nextPkgrel(tt.cur, tt.by)
		if tt.wantErr {
			if err == nil {
				t.Errorf("nextPkgrel(%q, %q) = %q, want error", tt.cur, tt.by, got)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("nextPkgrel(%q, %q) = %q, %v, want %q", tt.cur, tt.by, got, err, tt.want)
		}
	}
}

func TestRewritePkgrel(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"plain", "pkgname=foo\npkgver=1.0\npkgrel=1\narch=(any)\n", "pkgname=foo\npkgver=1.0\npkgrel=1.1\narch=(any)\n"},
		{"crlf", "pkgname=foo\r\npkgrel=1\r\narch=(any)\r\n", "pkgname=foo\r\npkgrel=1.1\r\narch=(any)\r\n"},
		{"trailing comment", "pkgrel=1 # rebuilt for openssl\n", "pkgrel=1.1 # rebuilt for openssl\n"},
		{"quoted", "pkgrel='1'\n", "pkgrel='1.1'\n"},
	}
	for _, tt := range tests {
		out, err := rewritePkgrel([]byte(tt.in), "0.1")
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if string(out) != tt.want {
			t.Errorf("%s: rewritePkgrel = %q, want %q", tt.name, out, tt.want)
		}
	}

	if _, err := rewritePkgrel([]byte("pkgname=foo\n"), "0.1"); err == nil {
		t.Error("rewritePkgrel without pkgrel should error")
	}
}

func TestBumpPkgrel(t *testing.T) {
	if _, err := exec.LookPath("makepkg"); err != nil {
		t.Skip("makepkg not on PATH")
	}

	dir := t.TempDir()
	pkgbuild := "pkgname=foo\npkgver=1.0\npkgrel=1\npkgdesc='test'\narch=(any)\n"
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(pkgbuild), 0o644); err != nil {
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

	bumped, err := BumpPkgrel(src, []string{"foo"}, "0.1", os.Stderr)
	if err != nil {
		t.Fatal(err)
	}
	if len(bumped) != 1 || bumped[0].Version() != "1.0-1.1" {
		t.Fatalf("bumped = %v, want [foo 1.0-1.1]", bumped)
	}
	data, err := os.ReadFile(filepath.Join(dir, "PKGBUILD"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "pkgrel=1.1") {
		t.Errorf("PKGBUILD not rewritten: %q", data)
	}

	bumped, err = BumpPkgrel(src, []string{"foo"}, "1", os.Stderr)
	if err != nil {
		t.Fatal(err)
	}
	if len(bumped) != 1 || bumped[0].Version() != "1.0-2" {
		t.Fatalf("bumped by 1 = %v, want [foo 1.0-2]", bumped)
	}

	if _, err := BumpPkgrel(src, []string{"nope"}, "0.1", os.Stderr); err == nil {
		t.Error("bumping an unknown package should error")
	}
}
