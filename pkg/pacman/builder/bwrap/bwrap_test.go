package bwrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder/internal/shellutil"
)

func TestNewBwrapBackend(t *testing.T) {
	b := New(builder.ResolvedConfig{})
	if b.Name() != "bwrap" {
		t.Errorf("Name = %q, want bwrap", b.Name())
	}
}

func TestNewBwrapPreservesUnlimitedZeroTimeout(t *testing.T) {
	b := New(builder.ResolvedConfig{})
	if b.timeout != 0 {
		t.Errorf("timeout = %s, want zero (unlimited)", b.timeout)
	}
}

func TestBwrapArgsOverlayAndUID(t *testing.T) {
	joined := strings.Join(bwrapArgs([]string{"/rootfs"}, "/s/upper", "/s/work", "/work/pkg", "", "0", "echo hi", nil), " ")
	for _, want := range []string{
		"--unshare-user", "--uid 0 --gid 0",
		"--overlay-src /rootfs --overlay /s/upper /s/work /",
		"--bind /work/pkg /build", "--chdir /build",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("bwrapArgs missing %q in %q", want, joined)
		}
	}
	if strings.Contains(joined, "/var/cache/pacman/pkg") {
		t.Errorf("empty cacheDir should not bind a package cache: %q", joined)
	}
	withCache := strings.Join(bwrapArgs([]string{"/rootfs"}, "/s/upper", "/s/work", "/work/pkg", "/pkgcache", "0", "echo hi", nil), " ")
	if !strings.Contains(withCache, "--bind /pkgcache /var/cache/pacman/pkg") {
		t.Errorf("cacheDir not bound: %q", withCache)
	}
	phase2 := strings.Join(bwrapArgs([]string{"/rootfs", "/s/deps"}, "/s/build", "/s/work2", "/work/pkg", "", "1000", "echo hi", nil), " ")
	if !strings.Contains(phase2, "--overlay-src /rootfs --overlay-src /s/deps --overlay /s/build /s/work2 /") {
		t.Errorf("phase 2 overlay stacking wrong: %q", phase2)
	}
}

func TestBwrapInstallBinds(t *testing.T) {
	script, binds, err := bwrapInstall([]string{"/cache/dep-1.0-1-x86_64.pkg.tar.zst"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(binds) != 1 || binds[0][0] != "/cache/dep-1.0-1-x86_64.pkg.tar.zst" {
		t.Fatalf("unexpected binds: %v", binds)
	}
	if !strings.Contains(script, "pacman -U --noconfirm "+shellutil.Quote(binds[0][1])) {
		t.Errorf("install command not substituted: %q", script)
	}
	if strings.Contains(script, "\n__INSTALL__\n") {
		t.Error("__INSTALL__ placeholder line not replaced")
	}
	if strings.Contains(script, "\n__EXTRA_REPOS__\n") {
		t.Error("__EXTRA_REPOS__ placeholder line not replaced")
	}
	if !strings.Contains(script, "DisableSandbox") {
		t.Error("deps script must disable the pacman download sandbox")
	}

	withRepo, _, err := bwrapInstall(nil, []builder.PacmanRepository{{Name: "myrepo", Server: "https://x/$repo/$arch"}})
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(withRepo, "<<'KAMISATO_EXTRA_REPO_EOF'"); n != 1 {
		t.Errorf("repository heredoc opener count = %d, want 1", n)
	}
}

func TestPrepareCacheDirCreatesDirectory(t *testing.T) {
	want := filepath.Join(t.TempDir(), "nested", "cache")
	got, err := prepareCacheDir(want)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("cache dir = %q, want %q", got, want)
	}
	if info, err := os.Stat(got); err != nil || !info.IsDir() {
		t.Fatalf("cache directory was not created: info=%v err=%v", info, err)
	}
}

func TestBwrapBuildScriptHasNoSyncdeps(t *testing.T) {
	if strings.Contains(bwrapBuildScript, "--syncdeps") {
		t.Error("build phase must not use --syncdeps")
	}
	if !strings.Contains(bwrapBuildScript, "makepkg") {
		t.Error("build phase must run makepkg")
	}
}

func TestBwrapBuildValidates(t *testing.T) {
	b := New(builder.ResolvedConfig{})
	if _, err := b.Build(t.Context(), builder.Spec{}); err == nil {
		t.Fatal("expected an error for an empty SrcDir")
	}
	if _, err := b.Build(t.Context(), builder.Spec{SrcDir: t.TempDir()}); err == nil {
		t.Fatal("expected an error when no rootfs is configured")
	}
}
