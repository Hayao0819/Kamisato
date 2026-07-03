//go:build linux

package builder

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestBwrapBuildIntegration drives the bwrap 2-phase backend against a real Arch
// host: phase 1 (uid 0 in a userns) installs base-devel into the overlay, phase 2
// (uid 1000) runs makepkg. It needs bwrap >= 0.11 with unprivileged user
// namespaces and overlay support, plus a pristine Arch rootfs pointed to by
// BWRAP_ROOTFS. It skips when that is unset so the off-host suite stays green.
func TestBwrapBuildIntegration(t *testing.T) {
	rootfs := os.Getenv("BWRAP_ROOTFS")
	if rootfs == "" {
		t.Skip("set BWRAP_ROOTFS to a pristine Arch rootfs to run the bwrap integration build")
	}

	src := t.TempDir()
	pkgbuild := strings.Join([]string{
		"pkgname=ayaka-bwrap-smoke",
		"pkgver=1.0",
		"pkgrel=1",
		"pkgdesc='bwrap 2-phase smoke test'",
		"arch=('x86_64')",
		"license=('MIT')",
		"options=('!debug')",
		"package() {",
		"  install -Dm644 /dev/null \"$pkgdir/usr/share/ayaka-bwrap-smoke/marker\"",
		"}",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(src, "PKGBUILD"), []byte(pkgbuild), 0o644); err != nil {
		t.Fatal(err)
	}

	// BWRAP_PKG_CACHE (optional) is a persistent pacman cache bind-mounted into
	// the sandbox so base-devel downloads resume across runs on a flaky mirror.
	b, err := New(KindBwrap, Options{
		BwrapRootfs:    rootfs,
		PacmanCacheDir: os.Getenv("BWRAP_PKG_CACHE"),
		Timeout:        15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("New(KindBwrap): %v", err)
	}

	res, err := b.Build(context.Background(), Spec{
		SrcDir:    src,
		OutDir:    src,
		Arch:      "x86_64",
		LogWriter: os.Stderr,
	})
	if err != nil {
		t.Fatalf("bwrap Build: %v", err)
	}
	if len(res.Packages) == 0 {
		t.Fatal("bwrap build produced no packages")
	}
	for _, p := range res.Packages {
		if !strings.Contains(p, ".pkg.tar.") {
			t.Fatalf("unexpected artifact %q", p)
		}
		if fi, err := os.Stat(p); err != nil || fi.Size() == 0 {
			t.Fatalf("built package %q missing or empty: %v", p, err)
		}
		t.Logf("bwrap produced %s", filepath.Base(p))
	}
}
