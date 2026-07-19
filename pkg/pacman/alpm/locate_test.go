package alpm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindCachedPackage(t *testing.T) {
	dir := t.TempDir()
	mk := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("foo-1.0-1-x86_64.pkg.tar.zst")
	mk("foo-1.0-1-x86_64.pkg.tar.zst.sig")  // signature must not be picked
	mk("foo-bar-2.0-1-x86_64.pkg.tar.zst")  // different package, must not collide
	mk("foo-1.0-1-1-x86_64.pkg.tar.zst")    // foo-1.0 ver 1-1: name+ver concat look-alike
	mk("foo-1.0-1-x86_64.pkg.tar.zst.part") // stale partial download

	got, ok := FindCachedPackage([]string{dir}, "foo", "1.0-1")
	if !ok {
		t.Fatal("expected to find the built package")
	}
	if filepath.Base(got) != "foo-1.0-1-x86_64.pkg.tar.zst" {
		t.Errorf("found %q, want the .pkg.tar.zst (not .sig, .part, foo-bar, or the concat look-alike)", filepath.Base(got))
	}

	if _, ok := FindCachedPackage([]string{dir}, "missing", "1.0-1"); ok {
		t.Error("a package with no cached file must report not found")
	}
	// Wrong version must not match a different cached version.
	if _, ok := FindCachedPackage([]string{dir}, "foo", "9.9-9"); ok {
		t.Error("a version with no cached file must report not found")
	}

	// Historical Kamisato filenames normalized an epoch ':' to '_'; the shared
	// metadata matcher keeps those cache entries discoverable.
	mk("epochpkg-2_1.0-1-x86_64.pkg.tar.zst")
	if got, ok := FindCachedPackage([]string{dir}, "epochpkg", "2:1.0-1"); !ok ||
		filepath.Base(got) != "epochpkg-2_1.0-1-x86_64.pkg.tar.zst" {
		t.Errorf("legacy epoch cache lookup = %q, %t", got, ok)
	}

	// Only a sidecar present (no completed file) must report not found, never the .part.
	only := t.TempDir()
	if err := os.WriteFile(filepath.Join(only, "bar-1-1-x86_64.pkg.tar.zst.part"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := FindCachedPackage([]string{only}, "bar", "1-1"); ok {
		t.Error("a lone .part sidecar must not be treated as a finished package")
	}
}

func TestFilterForeign(t *testing.T) {
	foreign := map[string]bool{"yay": true, "mytool": true}
	got := FilterForeign([]string{"glibc", "yay", "linux", "mytool"}, foreign)
	if len(got) != 2 || got[0] != "yay" || got[1] != "mytool" {
		t.Errorf("FilterForeign kept %v, want [yay mytool] (official packages dropped)", got)
	}
	if out := FilterForeign([]string{"glibc", "linux"}, foreign); len(out) != 0 {
		t.Errorf("all-official input should yield nothing, got %v", out)
	}
}
