//go:build linux

package bwrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
)

// TestBwrapBuildIntegration requires bwrap 0.11+, unprivileged overlayfs, and an
// Arch rootfs selected by BWRAP_ROOTFS.
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

	b := New(builder.ResolvedConfig{
		Timeout: 15 * time.Minute,
		Bwrap: builder.BwrapConfig{
			Rootfs:         rootfs,
			PacmanCacheDir: os.Getenv("BWRAP_PKG_CACHE"),
		},
	})

	res, err := b.Build(t.Context(), builder.Spec{
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
