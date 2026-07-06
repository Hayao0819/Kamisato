package raiou

import (
	"strings"
	"testing"
)

func TestPKGINFOBytesRoundTrip(t *testing.T) {
	in := NewPKGINFO()
	in.PkgName = "myrepo-keyring"
	in.PkgBase = "myrepo-keyring"
	in.PkgVer = "20260707-1"
	in.PkgDesc = "MyRepo PGP keyring"
	in.Arch = "any"
	in.BuildDate = 1751846400
	in.Packager = "MyRepo <repo@example.com>"
	in.Size = 4096
	in.License = []string{"GPL"}
	in.Depend = []string{"archlinux-keyring"}
	in.PkgType = "pkg"

	out, err := ParsePkginfoString(string(in.Bytes()))
	if err != nil {
		t.Fatalf("re-parse failed: %v", err)
	}
	if out.PkgName != in.PkgName || out.PkgVer != in.PkgVer || out.Arch != in.Arch {
		t.Errorf("scalar mismatch: got %+v", out)
	}
	if out.BuildDate != in.BuildDate || out.Size != in.Size {
		t.Errorf("numeric mismatch: builddate=%d size=%d", out.BuildDate, out.Size)
	}
	if len(out.Depend) != 1 || out.Depend[0] != "archlinux-keyring" {
		t.Errorf("depend mismatch: %v", out.Depend)
	}
	if out.PkgType != "pkg" {
		t.Errorf("pkgtype mismatch: %q", out.PkgType)
	}
}

func TestPKGINFOBytesOmitsEmptyScalars(t *testing.T) {
	in := NewPKGINFO()
	in.PkgName = "x"
	in.PkgVer = "1-1"
	in.Arch = "any"

	s := string(in.Bytes())
	if strings.Contains(s, "url =") || strings.Contains(s, "pkgdesc =") {
		t.Errorf("empty scalars should be omitted, got:\n%s", s)
	}
	// builddate and size are always present even at zero.
	if !strings.Contains(s, "builddate = 0") || !strings.Contains(s, "size = 0") {
		t.Errorf("builddate/size must always be written, got:\n%s", s)
	}
}
