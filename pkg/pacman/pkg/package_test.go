package pkg_test

import (
	"os"
	"path/filepath"
	"testing"

	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
)

const srcinfo = "pkgbase = foo\n" +
	"\tpkgver = 1.2.3\n" +
	"\tpkgrel = 2\n" +
	"\tepoch = 1\n" +
	"\tarch = x86_64\n" +
	"\n" +
	"pkgname = foo\n" +
	"\n" +
	"pkgname = foo-libs\n"

func writeSrcDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".SRCINFO"), []byte(srcinfo), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestOpenSourcePackage(t *testing.T) {
	dir := writeSrcDir(t)

	p, err := pkg.OpenSourcePackage(dir)
	if err != nil {
		t.Fatalf("OpenSourcePackage failed: %v", err)
	}

	if p.Base() != "foo" {
		t.Errorf("Base() = %q, want foo", p.Base())
	}
	// epoch:pkgver-pkgrel
	if p.Version() != "1:1.2.3-2" {
		t.Errorf("Version() = %q, want 1:1.2.3-2", p.Version())
	}
	names := p.Names()
	if len(names) != 2 || names[0] != "foo" || names[1] != "foo-libs" {
		t.Errorf("Names() = %v, want [foo foo-libs]", names)
	}
	if p.Dir() != dir {
		t.Errorf("Dir() = %q, want %q", p.Dir(), dir)
	}
}

func TestOpenSourcePackage_NoSrcinfo(t *testing.T) {
	dir := t.TempDir()
	if _, err := pkg.OpenSourcePackage(dir); err != pkg.ErrSRCINFONotFound {
		t.Errorf("OpenSourcePackage error = %v, want ErrSRCINFONotFound", err)
	}
}
