package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

func comparisonSource(t *testing.T, base, version string) *pkg.SourcePackage {
	t.Helper()
	dir := t.TempDir()
	srcinfo := fmt.Sprintf("pkgbase = %s\n\tpkgver = %s\n\tpkgrel = 1\n\tarch = x86_64\n\npkgname = %s\n", base, version, base)
	if err := os.WriteFile(filepath.Join(dir, ".SRCINFO"), []byte(srcinfo), 0o644); err != nil {
		t.Fatal(err)
	}
	source, err := pkg.OpenSourcePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func TestDiffPackages(t *testing.T) {
	sources := []*pkg.SourcePackage{
		comparisonSource(t, "newer", "2.0"),
		comparisonSource(t, "equal", "1.0"),
		comparisonSource(t, "older", "1.0"),
		comparisonSource(t, "missing", "1.0"),
	}
	remote := &RemoteRepo{Name: "test"}
	for _, info := range []raiou.PKGINFO{
		{PkgBase: "newer", PkgName: "newer", PkgVer: "1.0-1"},
		{PkgBase: "equal", PkgName: "equal", PkgVer: "1.0-1"},
		{PkgBase: "older", PkgName: "older", PkgVer: "2.0-1"},
	} {
		remote.Pkgs = append(remote.Pkgs, pkg.NewBinaryPackage(info.PkgName+".pkg.tar.zst", &info))
	}
	var got []string
	for _, source := range DiffPackages(sources, remote) {
		got = append(got, source.Base())
	}
	if want := []string{"newer", "missing"}; !slices.Equal(got, want) {
		t.Fatalf("DiffPackages = %v, want %v", got, want)
	}
}

func TestPrunablePackages(t *testing.T) {
	remote := &RemoteRepo{}
	for _, name := range []string{"foo", "bar", "bar-libs", "orphan"} {
		remote.Pkgs = append(remote.Pkgs, pkg.NewBinaryPackage(name+".pkg.tar.zst", &raiou.PKGINFO{PkgName: name}))
	}
	// Compare actual binary names, including split outputs, not only pkgbases.
	got := PrunablePackages([]string{"foo", "bar", "bar-libs"}, remote)
	if want := []string{"orphan"}; !slices.Equal(got, want) {
		t.Fatalf("PrunablePackages = %v, want %v", got, want)
	}
}
