package service

import (
	"reflect"
	"testing"
)

func TestSrcinfoBuildDeps(t *testing.T) {
	srcinfo := `pkgbase = foo
	pkgver = 1.0
	makedepends = cmake
	makedepends = git
	checkdepends = python-pytest
	depends = glibc
	depends_x86_64 = lib32-glibc
	depends_aarch64 = arm-thing
	optdepends = extra: does stuff
	provides = foo
pkgname = foo
	depends = glibc
	depends = bar>=2.0
`
	got := srcinfoBuildDeps([]byte(srcinfo), "x86_64")
	want := []string{"cmake", "git", "python-pytest", "glibc", "lib32-glibc", "bar>=2.0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("x86_64 deps:\n got %q\nwant %q", got, want)
	}

	// A different arch picks its own arch-specific deps and drops the other's.
	gotArm := srcinfoBuildDeps([]byte(srcinfo), "aarch64")
	for _, v := range gotArm {
		if v == "lib32-glibc" {
			t.Errorf("aarch64 build should not include x86_64-only dep: %q", gotArm)
		}
	}
	// optdepends and provides are never build dependencies.
	for _, v := range got {
		if v == "extra: does stuff" || v == "foo" {
			t.Errorf("optdepends/provides leaked into build deps: %q", got)
		}
	}
}

func TestSrcinfoBuildDepsEmpty(t *testing.T) {
	if got := srcinfoBuildDeps([]byte("pkgbase = x\n\tpkgver = 1\n"), "x86_64"); len(got) != 0 {
		t.Errorf("expected no deps, got %q", got)
	}
}
