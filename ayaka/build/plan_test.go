package build

import (
	"reflect"
	"testing"

	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

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

func TestBuildDepGraphPerArch(t *testing.T) {
	pkgs := []*pkg.SourcePackage{goSrc(t, "1.24"), kamisatoSrc(t, "0.1")}

	g32 := buildDepGraph(filterByArch(pkgs, "i686"), "i686")
	if got := g32.Deps("kamisato"); !reflect.DeepEqual(got, []string{"go"}) {
		t.Errorf("i686 deps of kamisato = %v, want [go]", got)
	}

	g64 := buildDepGraph(filterByArch(pkgs, "x86_64"), "x86_64")
	if got := g64.Deps("kamisato"); len(got) != 0 {
		t.Errorf("x86_64 deps of kamisato = %v, want none (go is external there)", got)
	}
}

func TestBuildDepGraphProvidesCannotShadowRealPackage(t *testing.T) {
	foo := srcinfoPkg(t, "pkgbase = foo\n\tpkgver = 1.0\n\tpkgrel = 1\n\tarch = x86_64\n\npkgname = foo\n")
	bar := srcinfoPkg(t, "pkgbase = bar\n\tpkgver = 1.0\n\tpkgrel = 1\n\tarch = x86_64\n\tprovides = foo\n\npkgname = bar\n")
	baz := srcinfoPkg(t, "pkgbase = baz\n\tpkgver = 1.0\n\tpkgrel = 1\n\tarch = x86_64\n\tmakedepends = foo\n\npkgname = baz\n")

	for _, pkgs := range [][]*pkg.SourcePackage{{foo, bar, baz}, {bar, foo, baz}} {
		g := buildDepGraph(pkgs, "x86_64")
		if got := g.Deps("baz"); !reflect.DeepEqual(got, []string{"foo"}) {
			t.Errorf("deps of baz = %v, want [foo] regardless of package order", got)
		}
	}
}

func TestComputePlanMakedependsCascade(t *testing.T) {
	pkgs := []*pkg.SourcePackage{goSrc(t, "1.25"), kamisatoSrc(t, "0.1")}
	rr := &repo.RemoteRepo{Name: "test", Pkgs: []*pkg.BinaryPackage{
		remoteBin("go", "1.24-1"),
		remoteBin("kamisato", "0.1-1"),
	}}

	plan, err := ComputePlan(pkgs, rr, "i686", CascadeMakeDepends, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"go", "kamisato"}; !reflect.DeepEqual(plan.Order, want) {
		t.Errorf("order = %v, want %v", plan.Order, want)
	}
	if plan.Reasons["go"] != "version" || plan.Reasons["kamisato"] != "makedepends" {
		t.Errorf("reasons = %v", plan.Reasons)
	}
	if want := []string{"kamisato"}; !reflect.DeepEqual(plan.BumpTargets, want) {
		t.Errorf("bump targets = %v, want %v", plan.BumpTargets, want)
	}

	// On x86_64 go is external, so kamisato does not rebuild for it.
	plan64, err := ComputePlan(pkgs, rr, "x86_64", CascadeMakeDepends, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan64.Order) != 0 {
		t.Errorf("x86_64 order = %v, want empty", plan64.Order)
	}
}

func TestComputePlanCascadeOff(t *testing.T) {
	pkgs := []*pkg.SourcePackage{goSrc(t, "1.25"), kamisatoSrc(t, "0.1")}
	rr := &repo.RemoteRepo{Name: "test", Pkgs: []*pkg.BinaryPackage{
		remoteBin("go", "1.24-1"),
		remoteBin("kamisato", "0.1-1"),
	}}

	plan, err := ComputePlan(pkgs, rr, "i686", CascadeOff, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"go"}; !reflect.DeepEqual(plan.Order, want) {
		t.Errorf("order = %v, want %v (off must not cascade)", plan.Order, want)
	}
}

func TestComputePlanEpochBumpCascades(t *testing.T) {
	pkgs := []*pkg.SourcePackage{
		srcinfoPkg(t, `pkgbase = go
	pkgver = 1.24
	pkgrel = 1
	epoch = 1
	arch = i686
	provides = go-pie

pkgname = go
`),
		kamisatoSrc(t, "0.1"),
	}
	rr := &repo.RemoteRepo{Name: "test", Pkgs: []*pkg.BinaryPackage{
		remoteBin("go", "1.24-1"),
		remoteBin("kamisato", "0.1-1"),
	}}

	plan, err := ComputePlan(pkgs, rr, "i686", CascadeMakeDepends, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"go", "kamisato"}; !reflect.DeepEqual(plan.Order, want) {
		t.Errorf("order = %v, want %v (epoch bump is a pkgver change)", plan.Order, want)
	}
}

func TestComputePlanPkgrelOnlyDoesNotCascade(t *testing.T) {
	pkgs := []*pkg.SourcePackage{
		srcinfoPkg(t, `pkgbase = go
	pkgver = 1.24
	pkgrel = 2
	arch = i686
	provides = go-pie

pkgname = go
`),
		kamisatoSrc(t, "0.1"),
	}
	rr := &repo.RemoteRepo{Name: "test", Pkgs: []*pkg.BinaryPackage{
		remoteBin("go", "1.24-1"),
		remoteBin("kamisato", "0.1-1"),
	}}

	plan, err := ComputePlan(pkgs, rr, "i686", CascadeMakeDepends, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"go"}; !reflect.DeepEqual(plan.Order, want) {
		t.Errorf("order = %v, want %v (pkgrel-only bump must not cascade)", plan.Order, want)
	}
	if len(plan.BumpTargets) != 0 {
		t.Errorf("bump targets = %v, want empty", plan.BumpTargets)
	}
}

func TestComputePlanSonameCascade(t *testing.T) {
	pkgs := []*pkg.SourcePackage{
		srcinfoPkg(t, "pkgbase = libfoo\n\tpkgver = 2.0\n\tpkgrel = 1\n\tarch = x86_64\n\npkgname = libfoo\n"),
		srcinfoPkg(t, "pkgbase = app\n\tpkgver = 1.0\n\tpkgrel = 1\n\tarch = x86_64\n\npkgname = app\n"),
	}
	rr := &repo.RemoteRepo{Name: "test", Pkgs: []*pkg.BinaryPackage{
		remoteBin("libfoo", "2.0-1", func(i *raiou.PKGINFO) {
			i.Provides = []string{"libfoo.so=2-64"}
		}),
		remoteBin("app", "1.0-1", func(i *raiou.PKGINFO) {
			i.Depend = []string{"libfoo.so=1-64", "libexternal.so=5-64", "glibc"}
		}),
	}}

	plan, err := ComputePlan(pkgs, rr, "x86_64", CascadeSoname, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"app"}; !reflect.DeepEqual(plan.Order, want) {
		t.Errorf("order = %v, want %v", plan.Order, want)
	}
	if plan.Reasons["app"] != "soname" {
		t.Errorf("reasons = %v", plan.Reasons)
	}
	if want := []string{"app"}; !reflect.DeepEqual(plan.BumpTargets, want) {
		t.Errorf("bump targets = %v, want %v", plan.BumpTargets, want)
	}
}

func TestComputePlanCascadeBoth(t *testing.T) {
	pkgs := []*pkg.SourcePackage{
		goSrc(t, "1.25"),
		kamisatoSrc(t, "0.1"),
		srcinfoPkg(t, "pkgbase = libfoo\n\tpkgver = 2.0\n\tpkgrel = 1\n\tarch = i686\n\npkgname = libfoo\n"),
		srcinfoPkg(t, "pkgbase = app\n\tpkgver = 1.0\n\tpkgrel = 1\n\tarch = i686\n\npkgname = app\n"),
	}
	rr := &repo.RemoteRepo{Name: "test", Pkgs: []*pkg.BinaryPackage{
		remoteBin("go", "1.24-1"),
		remoteBin("kamisato", "0.1-1"),
		remoteBin("libfoo", "2.0-1", func(i *raiou.PKGINFO) {
			i.Provides = []string{"libfoo.so=2-64"}
		}),
		remoteBin("app", "1.0-1", func(i *raiou.PKGINFO) {
			i.Depend = []string{"libfoo.so=1-64"}
		}),
	}}

	plan, err := ComputePlan(pkgs, rr, "i686", CascadeBoth, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Reasons["kamisato"] != "makedepends" || plan.Reasons["app"] != "soname" {
		t.Errorf("reasons = %v, want both cascades active", plan.Reasons)
	}
	if want := []string{"app", "kamisato"}; !reflect.DeepEqual(plan.BumpTargets, want) {
		t.Errorf("bump targets = %v, want %v", plan.BumpTargets, want)
	}
}

func TestComputePlanEmpty(t *testing.T) {
	pkgs := []*pkg.SourcePackage{kamisatoSrc(t, "0.1")}
	rr := &repo.RemoteRepo{Name: "test", Pkgs: []*pkg.BinaryPackage{
		remoteBin("kamisato", "0.1-1"),
	}}
	plan, err := ComputePlan(pkgs, rr, "x86_64", CascadeMakeDepends, 4, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Order) != 0 || len(plan.Buckets) != 0 || len(plan.BumpTargets) != 0 {
		t.Errorf("plan = %+v, want empty", plan)
	}
}

func TestComputePlanBuckets(t *testing.T) {
	// go+kamisato form one component; linux-nost is independent and heavy.
	pkgs := []*pkg.SourcePackage{
		goSrc(t, "1.25"),
		kamisatoSrc(t, "0.1"),
		srcinfoPkg(t, "pkgbase = linux-nost\n\tpkgver = 6.10\n\tpkgrel = 1\n\tarch = i686\n\npkgname = linux-nost\n"),
	}
	rr := &repo.RemoteRepo{Name: "test", Pkgs: []*pkg.BinaryPackage{
		remoteBin("kamisato", "0.1-1"),
	}}

	costs := map[string]float64{"linux-nost": 300, "go": 60, "kamisato": 5}
	plan, err := ComputePlan(pkgs, rr, "i686", CascadeMakeDepends, 2, costs)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"linux-nost"}, {"go", "kamisato"}}
	if !reflect.DeepEqual(plan.Buckets, want) {
		t.Errorf("buckets = %v, want %v", plan.Buckets, want)
	}
}

func TestPackBucketsFewerComponentsThanWorkers(t *testing.T) {
	comps := [][]string{{"a"}}
	got := packBuckets(comps, []string{"a"}, 4, nil)
	if len(got) != 1 || !reflect.DeepEqual(got[0], []string{"a"}) {
		t.Errorf("buckets = %v, want [[a]]", got)
	}
}
