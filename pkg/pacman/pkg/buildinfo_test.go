package pkg_test

import (
	"archive/tar"
	"bytes"
	"errors"
	"testing"

	"github.com/klauspost/compress/zstd"

	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
)

// buildPkg assembles a .pkg.tar.zst carrying the given top-level members, so a
// test can include or omit .BUILDINFO.
func buildPkg(t *testing.T, members map[string]string) []byte {
	t.Helper()
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	// A fixed order keeps .PKGINFO ahead of .BUILDINFO, matching a real package.
	for _, name := range []string{".PKGINFO", ".BUILDINFO"} {
		body, ok := members[name]
		if !ok {
			continue
		}
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	var zBuf bytes.Buffer
	zw, err := zstd.NewWriter(&zBuf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := zw.Write(tarBuf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return zBuf.Bytes()
}

func TestReadBuildInfo(t *testing.T) {
	data := buildPkg(t, map[string]string{
		".PKGINFO":   "pkgname = foo\npkgver = 1.0-1\narch = x86_64\n",
		".BUILDINFO": "format = 2\nbuilddir = /build\n",
	})
	bi, err := pkg.ReadBuildInfo(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ReadBuildInfo: %v", err)
	}
	if bi.BuildDir != "/build" {
		t.Errorf("BuildDir = %q, want /build", bi.BuildDir)
	}
}

func TestReadBuildInfoMissing(t *testing.T) {
	data := buildPkg(t, map[string]string{
		".PKGINFO": "pkgname = foo\npkgver = 1.0-1\narch = x86_64\n",
	})
	if _, err := pkg.ReadBuildInfo(bytes.NewReader(data)); !errors.Is(err, pkg.ErrBuildInfoNotFound) {
		t.Errorf("ReadBuildInfo error = %v, want ErrBuildInfoNotFound", err)
	}
}
