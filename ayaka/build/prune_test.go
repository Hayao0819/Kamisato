package build_test

import (
	"slices"
	"testing"

	"github.com/Hayao0819/Kamisato/ayaka/build"
	"github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

func TestPrunablePackages(t *testing.T) {
	rr := &repo.RemoteRepo{Pkgs: []*pkg.BinaryPackage{
		pkg.NewBinaryPackage("foo-1-1-x86_64.pkg.tar.zst", &raiou.PKGINFO{PkgName: "foo"}),
		pkg.NewBinaryPackage("bar-1-1-x86_64.pkg.tar.zst", &raiou.PKGINFO{PkgName: "bar"}),
		pkg.NewBinaryPackage("bar-libs-1-1-x86_64.pkg.tar.zst", &raiou.PKGINFO{PkgName: "bar-libs"}),
		pkg.NewBinaryPackage("orphan-1-1-x86_64.pkg.tar.zst", &raiou.PKGINFO{PkgName: "orphan"}),
	}}
	// The source repo still provides foo and the bar split package, but not orphan.
	got := build.PrunablePackages([]string{"foo", "bar", "bar-libs"}, rr)
	if !slices.Equal(got, []string{"orphan"}) {
		t.Fatalf("PrunablePackages = %v, want [orphan]", got)
	}
}
