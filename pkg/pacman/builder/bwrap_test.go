package builder

import (
	"context"
	"strings"
	"testing"
)

func TestNewBwrapBackend(t *testing.T) {
	b, err := New(KindBwrap, Options{})
	if err != nil {
		t.Fatalf("New(KindBwrap): %v", err)
	}
	if b.Name() != "bwrap" {
		t.Errorf("Name = %q, want bwrap", b.Name())
	}
}

func TestBwrapArgsOverlayAndUID(t *testing.T) {
	joined := strings.Join(bwrapArgs("/rootfs", "/s/upper", "/s/work", "/work/pkg", "", "0", "echo hi", nil), " ")
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
	// With a cache dir set, it is bind-mounted at the pacman cache path.
	withCache := strings.Join(bwrapArgs("/rootfs", "/s/upper", "/s/work", "/work/pkg", "/pkgcache", "0", "echo hi", nil), " ")
	if !strings.Contains(withCache, "--bind /pkgcache /var/cache/pacman/pkg") {
		t.Errorf("cacheDir not bound: %q", withCache)
	}
}

func TestBwrapInstallBinds(t *testing.T) {
	script, binds := bwrapInstall([]string{"/cache/dep-1.0-1-x86_64.pkg.tar.zst"}, nil)
	if len(binds) != 1 || binds[0][0] != "/cache/dep-1.0-1-x86_64.pkg.tar.zst" {
		t.Fatalf("unexpected binds: %v", binds)
	}
	if !strings.Contains(script, "pacman -U --noconfirm "+shellQuote(binds[0][1])) {
		t.Errorf("install command not substituted: %q", script)
	}
	if strings.Contains(script, "__INSTALL__") {
		t.Error("__INSTALL__ placeholder not replaced")
	}
	// With no extra repos the placeholder collapses to nothing.
	if strings.Contains(script, "__EXTRA_REPOS__") {
		t.Error("__EXTRA_REPOS__ placeholder not replaced")
	}
	// The dep phase must disable pacman's nested download sandbox.
	if !strings.Contains(script, "DisableSandbox") {
		t.Error("deps script must disable the pacman download sandbox")
	}
}

func TestBwrapBuildScriptHasNoSyncdeps(t *testing.T) {
	// Deps are installed in phase 1, so the non-root build must not use --syncdeps.
	if strings.Contains(bwrapBuildScript, "--syncdeps") {
		t.Error("build phase must not use --syncdeps")
	}
	if !strings.Contains(bwrapBuildScript, "makepkg") {
		t.Error("build phase must run makepkg")
	}
}

func TestBwrapBuildValidates(t *testing.T) {
	b := newBwrapBackend(Options{})
	if _, err := b.Build(context.Background(), Spec{}); err == nil {
		t.Fatal("expected an error for an empty SrcDir")
	}
	if _, err := b.Build(context.Background(), Spec{SrcDir: t.TempDir()}); err == nil {
		t.Fatal("expected an error when no rootfs is configured")
	}
}
