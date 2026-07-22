package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

// srcinfoPkg opens a SourcePackage from a raw .SRCINFO body.
func srcinfoPkg(t *testing.T, srcinfo string) *pkg.SourcePackage {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".SRCINFO"), []byte(srcinfo), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := pkg.OpenSourcePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// srcPkg writes a minimal .SRCINFO (pkgrel is fixed at 1, so Version is ver-1)
// and opens it as a SourcePackage. When no names are given, pkgbase is used.
func srcPkg(t *testing.T, base, ver string, names ...string) *pkg.SourcePackage {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "pkgbase = %s\n\tpkgver = %s\n\tpkgrel = 1\n\tarch = x86_64\n\n", base, ver)
	if len(names) == 0 {
		names = []string{base}
	}
	for _, n := range names {
		fmt.Fprintf(&b, "pkgname = %s\n", n)
	}
	return srcinfoPkg(t, b.String())
}

func remoteBin(base, ver string, mutate ...func(*raiou.PKGINFO)) *pkg.BinaryPackage {
	info := raiou.NewPKGINFO()
	info.PkgBase = base
	info.PkgName = base
	info.PkgVer = ver
	for _, m := range mutate {
		m(info)
	}
	return pkg.NewBinaryPackage(base+"-"+ver+".pkg.tar.zst", info)
}

func bases(pkgs []*pkg.SourcePackage) []string {
	out := make([]string, len(pkgs))
	for i, p := range pkgs {
		out[i] = p.Base()
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// goSrc self-hosts go for 32bit only; kamisatoSrc makedepends on it (via its
// go-pie provide, exercising provider resolution) and also builds for x86_64.
func goSrc(t *testing.T, ver string) *pkg.SourcePackage {
	return srcinfoPkg(t, `pkgbase = go
	pkgver = `+ver+`
	pkgrel = 1
	arch = i686
	provides = go-pie

pkgname = go
`)
}

func kamisatoSrc(t *testing.T, ver string) *pkg.SourcePackage {
	return srcinfoPkg(t, `pkgbase = kamisato
	pkgver = `+ver+`
	pkgrel = 1
	arch = x86_64
	arch = i686
	makedepends = go-pie

pkgname = kamisato
`)
}

func TestSelectPackages(t *testing.T) {
	foo := srcPkg(t, "foo", "1.0", "foo", "foo-libs")
	bar := srcPkg(t, "bar", "1.0")
	baz := srcPkg(t, "baz", "1.0")
	all := []*pkg.SourcePackage{foo, bar, baz}

	tests := []struct {
		name  string
		names []string
		want  []string
	}{
		{"empty selects all", nil, []string{"foo", "bar", "baz"}},
		{"match by pkgbase", []string{"bar"}, []string{"bar"}},
		{"match by sub-package name", []string{"foo-libs"}, []string{"foo"}},
		{"unknown name skipped", []string{"nope"}, nil},
		{"multiple names keep arg order", []string{"baz", "foo"}, []string{"baz", "foo"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bases(SelectPackages(all, tt.names))
			if !equalStrings(got, tt.want) {
				t.Errorf("SelectPackages(%v) = %v, want %v", tt.names, got, tt.want)
			}
		})
	}
}

func TestOrderByDeps(t *testing.T) {
	pkgs := []*pkg.SourcePackage{kamisatoSrc(t, "0.1"), goSrc(t, "1.24")}
	got := bases(OrderByDeps(pkgs, "i686"))
	if !equalStrings(got, []string{"go", "kamisato"}) {
		t.Errorf("OrderByDeps = %v, want [go kamisato]", got)
	}

	// A cycle keeps the incoming order instead of failing the build.
	a := srcinfoPkg(t, "pkgbase = a\n\tpkgver = 1\n\tpkgrel = 1\n\tarch = x86_64\n\tmakedepends = b\n\npkgname = a\n")
	b := srcinfoPkg(t, "pkgbase = b\n\tpkgver = 1\n\tpkgrel = 1\n\tarch = x86_64\n\tmakedepends = a\n\npkgname = b\n")
	got = bases(OrderByDeps([]*pkg.SourcePackage{a, b}, "x86_64"))
	if !equalStrings(got, []string{"a", "b"}) {
		t.Errorf("OrderByDeps with cycle = %v, want incoming order [a b]", got)
	}
}

func TestDiffPackages(t *testing.T) {
	src := []*pkg.SourcePackage{
		srcPkg(t, "newer", "2.0"),   // remote 1.0-1 -> build
		srcPkg(t, "equal", "1.0"),   // remote 1.0-1 -> skip
		srcPkg(t, "older", "1.0"),   // remote 2.0-1 -> skip
		srcPkg(t, "missing", "1.0"), // not in remote -> build
	}
	rr := &RemoteRepo{
		Name: "test",
		Pkgs: []*pkg.BinaryPackage{
			remoteBin("newer", "1.0-1"),
			remoteBin("equal", "1.0-1"),
			remoteBin("older", "2.0-1"),
		},
	}

	got := DiffPackages(src, rr)
	want := []string{"newer", "missing"}
	if !equalStrings(bases(got), want) {
		t.Errorf("DiffPackages = %v, want %v", bases(got), want)
	}
}

func TestPrunablePackages(t *testing.T) {
	rr := &RemoteRepo{Pkgs: []*pkg.BinaryPackage{
		pkg.NewBinaryPackage("foo-1-1-x86_64.pkg.tar.zst", &raiou.PKGINFO{PkgName: "foo"}),
		pkg.NewBinaryPackage("bar-1-1-x86_64.pkg.tar.zst", &raiou.PKGINFO{PkgName: "bar"}),
		pkg.NewBinaryPackage("bar-libs-1-1-x86_64.pkg.tar.zst", &raiou.PKGINFO{PkgName: "bar-libs"}),
		pkg.NewBinaryPackage("orphan-1-1-x86_64.pkg.tar.zst", &raiou.PKGINFO{PkgName: "orphan"}),
	}}
	// The source repo still provides foo and the bar split package, but not orphan.
	got := PrunablePackages([]string{"foo", "bar", "bar-libs"}, rr)
	if !equalStrings(got, []string{"orphan"}) {
		t.Fatalf("PrunablePackages = %v, want [orphan]", got)
	}
}

func TestBuildDepGraphPerArch(t *testing.T) {
	pkgs := []*pkg.SourcePackage{goSrc(t, "1.24"), kamisatoSrc(t, "0.1")}

	g32 := BuildDepGraph(FilterByArch(pkgs, "i686"), "i686")
	if got := g32.Deps("kamisato"); !reflect.DeepEqual(got, []string{"go"}) {
		t.Errorf("i686 deps of kamisato = %v, want [go]", got)
	}

	g64 := BuildDepGraph(FilterByArch(pkgs, "x86_64"), "x86_64")
	if got := g64.Deps("kamisato"); len(got) != 0 {
		t.Errorf("x86_64 deps of kamisato = %v, want none (go is external there)", got)
	}
}

func TestBuildDepGraphProvidesCannotShadowRealPackage(t *testing.T) {
	foo := srcinfoPkg(t, "pkgbase = foo\n\tpkgver = 1.0\n\tpkgrel = 1\n\tarch = x86_64\n\npkgname = foo\n")
	bar := srcinfoPkg(t, "pkgbase = bar\n\tpkgver = 1.0\n\tpkgrel = 1\n\tarch = x86_64\n\tprovides = foo\n\npkgname = bar\n")
	baz := srcinfoPkg(t, "pkgbase = baz\n\tpkgver = 1.0\n\tpkgrel = 1\n\tarch = x86_64\n\tmakedepends = foo\n\npkgname = baz\n")

	for _, pkgs := range [][]*pkg.SourcePackage{{foo, bar, baz}, {bar, foo, baz}} {
		g := BuildDepGraph(pkgs, "x86_64")
		if got := g.Deps("baz"); !reflect.DeepEqual(got, []string{"foo"}) {
			t.Errorf("deps of baz = %v, want [foo] regardless of package order", got)
		}
	}
}
