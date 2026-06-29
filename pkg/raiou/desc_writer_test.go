package raiou

import (
	"strings"
	"testing"
)

func descBlock(key string, vals ...string) string {
	return "%" + key + "%\n" + strings.Join(vals, "\n") + "\n\n"
}

func TestDescFromPkginfoBytes(t *testing.T) {
	info := NewPKGINFO()
	info.PkgName = "foo"
	info.PkgVer = "1.0-1"
	info.PkgDesc = "a   test\twith spaces" // runs of whitespace collapse to one space
	info.Arch = "x86_64"
	info.Size = 100
	info.BuildDate = 1700000000
	info.Packager = "me"
	info.URL = "http://example.com"
	info.License = []string{"MIT"}
	info.Depend = []string{"bar>1", "baz"}

	got := string(DescFromPkginfo(info, "foo-1.0-1-x86_64.pkg.tar.zst", 200, "deadbeef").Bytes())

	want := descBlock("FILENAME", "foo-1.0-1-x86_64.pkg.tar.zst") +
		descBlock("NAME", "foo") +
		descBlock("VERSION", "1.0-1") +
		descBlock("DESC", "a test with spaces") +
		descBlock("CSIZE", "200") +
		descBlock("ISIZE", "100") +
		descBlock("SHA256SUM", "deadbeef") +
		descBlock("URL", "http://example.com") +
		descBlock("LICENSE", "MIT") +
		descBlock("ARCH", "x86_64") +
		descBlock("BUILDDATE", "1700000000") +
		descBlock("PACKAGER", "me") +
		descBlock("DEPENDS", "bar>1", "baz")

	if got != want {
		t.Errorf("desc bytes mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestDescRoundTrip(t *testing.T) {
	info := NewPKGINFO()
	info.PkgName = "pac"
	info.PkgBase = "pac-base"
	info.PkgVer = "2:3.4-5"
	info.PkgDesc = "package manager"
	info.Arch = "x86_64"
	info.Size = 4096
	info.BuildDate = 1710000000
	info.Packager = "Dev <dev@example.com>"
	info.License = []string{"GPL2", "GPL3"}
	info.Depend = []string{"glibc", "curl>=7"}
	info.OptDepend = []string{"foo: bar"}
	info.Provides = []string{"libpac"}

	desc, err := ParseDescString(string(DescFromPkginfo(info, "pac-2:3.4-5-x86_64.pkg.tar.zst", 1234, "cafe").Bytes()))
	if err != nil {
		t.Fatalf("ParseDescString: %v", err)
	}

	if desc.Name != "pac" || desc.Base != "pac-base" || desc.Version != "2:3.4-5" {
		t.Errorf("identity fields wrong: %+v", desc)
	}
	if desc.FileName != "pac-2:3.4-5-x86_64.pkg.tar.zst" {
		t.Errorf("filename = %q", desc.FileName)
	}
	if desc.CSize != 1234 || desc.ISize != 4096 {
		t.Errorf("sizes wrong: csize=%d isize=%d", desc.CSize, desc.ISize)
	}
	if desc.SHA256SUM != "cafe" {
		t.Errorf("sha256 = %q", desc.SHA256SUM)
	}
	if strings.Join(desc.License, ",") != "GPL2,GPL3" {
		t.Errorf("license = %v", desc.License)
	}
	if strings.Join(desc.Depends, ",") != "glibc,curl>=7" {
		t.Errorf("depends = %v", desc.Depends)
	}
	if strings.Join(desc.Provides, ",") != "libpac" {
		t.Errorf("provides = %v", desc.Provides)
	}
}

func TestDescOmitsEmptyFields(t *testing.T) {
	info := NewPKGINFO()
	info.PkgName = "min"
	info.PkgVer = "1-1"
	info.Arch = "any"
	// no desc, url, license, deps, builddate, isize

	got := string(DescFromPkginfo(info, "min-1-1-any.pkg.tar.zst", 50, "ab").Bytes())

	for _, absent := range []string{"%BASE%", "%DESC%", "%URL%", "%LICENSE%", "%BUILDDATE%", "%DEPENDS%", "%PGPSIG%", "%MD5SUM%", "%GROUPS%"} {
		if strings.Contains(got, absent) {
			t.Errorf("expected %s to be omitted, got:\n%s", absent, got)
		}
	}
	// CSIZE and ISIZE are always present, even at 0 (matches repo-add on a
	// payload-less metapackage).
	for _, present := range []string{"%FILENAME%", "%NAME%", "%VERSION%", "%CSIZE%", "%ISIZE%", "%SHA256SUM%", "%ARCH%"} {
		if !strings.Contains(got, present) {
			t.Errorf("expected %s to be present, got:\n%s", present, got)
		}
	}
	if !strings.Contains(got, "%ISIZE%\n0\n") {
		t.Errorf("expected ISIZE to be emitted as 0, got:\n%s", got)
	}
}
