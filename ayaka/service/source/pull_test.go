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

func TestIsAURURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://aur.archlinux.org/ckbcomp.git", true},
		{"ssh://aur@aur.archlinux.org/ckbcomp.git", true},
		{"aur@aur.archlinux.org:ckbcomp.git", true},
		{"https://github.com/FascodeNet/archlinux32-keyring.git", false},
		{"git@github.com:FascodeNet/alterlinux-filesystem.git", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isAURURL(tt.url); got != tt.want {
			t.Errorf("isAURURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// setupMirror builds an origin repo with a v1 PKGBUILD and a detached-HEAD
// clone of it (the state actions/checkout leaves submodules in), then advances
// the origin to v2.
func setupMirror(t *testing.T) (originDir, cloneDir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	origin := t.TempDir()
	gitIn(t, origin, "init", "-q", "-b", "master")
	writeMirrorFiles(t, origin, "1.0")
	gitIn(t, origin, "add", ".")
	gitIn(t, origin, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "v1")

	parent := t.TempDir()
	clone := filepath.Join(parent, "foo")
	gitIn(t, parent, "clone", "-q", origin, clone)
	gitIn(t, clone, "checkout", "-q", "--detach", "HEAD")

	writeMirrorFiles(t, origin, "2.0")
	gitIn(t, origin, "add", ".")
	gitIn(t, origin, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "v2")
	return origin, clone
}

func writeMirrorFiles(t *testing.T, dir, ver string) {
	t.Helper()
	srcinfo := "pkgbase = foo\n\tpkgver = " + ver + "\n\tpkgrel = 1\n\tarch = any\n\npkgname = foo\n"
	if err := os.WriteFile(filepath.Join(dir, ".SRCINFO"), []byte(srcinfo), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPullPackages(t *testing.T) {
	_, clone := setupMirror(t)
	p, err := pkg.OpenSourcePackage(clone)
	if err != nil {
		t.Fatal(err)
	}
	src := &repo.SourceRepo{Config: &repo.SrcConfig{Name: "test"}, Pkgs: []*pkg.SourcePackage{p}}

	pulled, err := PullPackages(t.Context(), src, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(pulled) != 1 || pulled[0].Version() != "2.0-1" {
		t.Fatalf("pulled = %v, want [foo 2.0-1]", pulled)
	}
}

func TestPullPackagesDirtyRefusedUnlessForced(t *testing.T) {
	_, clone := setupMirror(t)
	p, err := pkg.OpenSourcePackage(clone)
	if err != nil {
		t.Fatal(err)
	}
	writeMirrorFiles(t, clone, "9.9")
	src := &repo.SourceRepo{Config: &repo.SrcConfig{Name: "test"}, Pkgs: []*pkg.SourcePackage{p}}

	if _, err := PullPackages(t.Context(), src, []string{"foo"}, false); err == nil ||
		!strings.Contains(err.Error(), "local changes") {
		t.Errorf("dirty checkout should be refused, got %v", err)
	}

	pulled, err := PullPackages(t.Context(), src, []string{"foo"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(pulled) != 1 || pulled[0].Version() != "2.0-1" {
		t.Fatalf("forced pull = %v, want [foo 2.0-1]", pulled)
	}
}

func TestPullPackagesRejectsNonCheckout(t *testing.T) {
	dir := t.TempDir()
	srcinfo := "pkgbase = foo\n\tpkgver = 1.0\n\tpkgrel = 1\n\tarch = any\n\npkgname = foo\n"
	if err := os.WriteFile(filepath.Join(dir, ".SRCINFO"), []byte(srcinfo), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := pkg.OpenSourcePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	src := &repo.SourceRepo{Config: &repo.SrcConfig{Name: "test"}, Pkgs: []*pkg.SourcePackage{p}}

	if _, err := PullPackages(t.Context(), src, []string{"foo"}, false); err == nil ||
		!strings.Contains(err.Error(), "not a git checkout") {
		t.Errorf("naming a non-checkout should error, got %v", err)
	}
	// With no names it is silently excluded instead.
	pulled, err := PullPackages(t.Context(), src, nil, false)
	if err != nil || len(pulled) != 0 {
		t.Errorf("all-mirrors pull = %v, %v; want empty, nil", pulled, err)
	}
}
