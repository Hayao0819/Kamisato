package raiou

import "testing"

func TestParseBuildinfo(t *testing.T) {
	const body = "format = 2\n" +
		"pkgname = foo\n" +
		"pkgver = 1.0-1\n" +
		"builddir = /build\n" +
		"startdir = /startdir\n" +
		"buildenv = ccache\n" +
		"buildenv = color\n"
	bi, err := ParseBuildinfoString(body)
	if err != nil {
		t.Fatalf("ParseBuildinfoString: %v", err)
	}
	if bi.Format != "2" {
		t.Errorf("Format = %q, want 2", bi.Format)
	}
	if bi.BuildDir != "/build" {
		t.Errorf("BuildDir = %q, want /build", bi.BuildDir)
	}
}

func TestParseBuildinfoUntrustedBuildDir(t *testing.T) {
	bi, err := ParseBuildinfoString("builddir = /home/mallory/pkg\n")
	if err != nil {
		t.Fatalf("ParseBuildinfoString: %v", err)
	}
	if bi.BuildDir != "/home/mallory/pkg" {
		t.Errorf("BuildDir = %q, want /home/mallory/pkg", bi.BuildDir)
	}
}
