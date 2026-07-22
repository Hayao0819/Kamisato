package pkg

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParseArtifact(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		filename    string
		wantArchive bool
		wantSig     bool
	}{
		{name: "zstd", filename: "foo-1.0-1-x86_64.pkg.tar.zst", wantArchive: true},
		{name: "xz", filename: "foo-1.0-1-x86_64.pkg.tar.xz", wantArchive: true},
		{name: "gzip", filename: "foo-1.0-1-x86_64.pkg.tar.gz", wantArchive: true},
		{name: "bzip2", filename: "foo-1.0-1-x86_64.pkg.tar.bz2", wantArchive: true},
		{name: "lrzip", filename: "foo-1.0-1-x86_64.pkg.tar.lrz", wantArchive: true},
		{name: "lzo", filename: "foo-1.0-1-x86_64.pkg.tar.lzo", wantArchive: true},
		{name: "lz4", filename: "foo-1.0-1-x86_64.pkg.tar.lz4", wantArchive: true},
		{name: "lzip", filename: "foo-1.0-1-x86_64.pkg.tar.lz", wantArchive: true},
		{name: "compress", filename: "foo-1.0-1-x86_64.pkg.tar.Z", wantArchive: true},
		{name: "uncompressed", filename: "foo-1.0-1-x86_64.pkg.tar", wantArchive: true},
		{name: "signature", filename: "foo-1.0-1-x86_64.pkg.tar.zst.sig", wantSig: true},
		{name: "minimal build output", filename: "result.pkg.tar.zst", wantArchive: true},
		{name: "unsupported suffix", filename: "foo-1.0-1-x86_64.pkg.tar.zip"},
		{name: "partial download", filename: "foo-1.0-1-x86_64.pkg.tar.zst.part"},
		{name: "double signature", filename: "foo-1.0-1-x86_64.pkg.tar.zst.sig.sig"},
		{name: "suffix only", filename: ".pkg.tar.zst"},
		{name: "embedded marker", filename: "foo.pkg.tar.zst.backup"},
		{name: "slash path", filename: "dir/foo-1.0-1-x86_64.pkg.tar.zst"},
		{name: "backslash path", filename: `dir\foo-1.0-1-x86_64.pkg.tar.zst`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			artifact, err := ParseArtifact(test.filename)
			wantOK := test.wantArchive || test.wantSig
			if (err == nil) != wantOK {
				t.Fatalf("ParseArtifact(%q) error = %v, want success %t", test.filename, err, wantOK)
			}
			if !wantOK {
				if !errors.Is(err, ErrInvalidArtifact) {
					t.Fatalf("error = %v, want ErrInvalidArtifact", err)
				}
				return
			}
			if artifact.IsSignature() != test.wantSig {
				t.Errorf("IsSignature = %t, want %t", artifact.IsSignature(), test.wantSig)
			}
			if IsArchive(test.filename) != test.wantArchive {
				t.Errorf("IsArchive(%q) = %t, want %t", test.filename, IsArchive(test.filename), test.wantArchive)
			}
			if !IsArtifact(test.filename) {
				t.Errorf("IsArtifact(%q) = false", test.filename)
			}
		})
	}
}

func TestArtifactCoordinates(t *testing.T) {
	t.Parallel()
	artifact, err := ParseArtifact("python-foo-2:1.2.3-4-any.pkg.tar.zst.sig")
	if err != nil {
		t.Fatal(err)
	}
	coordinates, err := artifact.Coordinates()
	if err != nil {
		t.Fatal(err)
	}
	if coordinates.Name != "python-foo" || coordinates.Version != "2:1.2.3" ||
		coordinates.Release != "4" || coordinates.Arch != "any" {
		t.Fatalf("coordinates = %#v", coordinates)
	}
	if !coordinates.IsAny() || !IsAny(artifact.Filename()) {
		t.Error("architecture-independent artifact was not recognized")
	}
	if !coordinates.MatchesMetadata("python-foo", "2:1.2.3-4", "any") {
		t.Error("coordinates do not match equivalent PKGINFO")
	}
	if artifact.ArchiveFilename() != "python-foo-2:1.2.3-4-any.pkg.tar.zst" {
		t.Errorf("ArchiveFilename = %q", artifact.ArchiveFilename())
	}
	if artifact.SignatureFilename() != artifact.Filename() {
		t.Errorf("SignatureFilename = %q, want %q", artifact.SignatureFilename(), artifact.Filename())
	}

	legacy, err := ParseArtifact("python-foo-2_1.2.3-4-any.pkg.tar.zst")
	if err != nil {
		t.Fatal(err)
	}
	legacyCoordinates, err := legacy.Coordinates()
	if err != nil || !legacyCoordinates.MatchesMetadata("python-foo", "2:1.2.3-4", "any") {
		t.Errorf("legacy epoch mismatch: coordinates=%+v error=%v", legacyCoordinates, err)
	}
}

func TestCoordinatesRejectsUnstructuredArtifact(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"result.pkg.tar.zst",
		"foo-1-1-bad.arch.pkg.tar.zst",
		"foo--1-x86_64.pkg.tar.zst",
		"bad name-1-1-x86_64.pkg.tar.zst",
		".hidden-1-1-x86_64.pkg.tar.zst",
	} {
		artifact, err := ParseArtifact(name)
		if err != nil {
			t.Fatalf("ParseArtifact(%q): %v", name, err)
		}
		if _, err := artifact.Coordinates(); !errors.Is(err, ErrInvalidArtifact) {
			t.Errorf("Coordinates(%q) error = %v, want ErrInvalidArtifact", name, err)
		}
	}
}

func TestSupportedArchiveSuffixesReturnsCopy(t *testing.T) {
	t.Parallel()
	first := SupportedArchiveSuffixes()
	if len(first) == 0 {
		t.Fatal("SupportedArchiveSuffixes returned no formats")
	}
	first[0] = ".changed"
	if second := SupportedArchiveSuffixes(); second[0] == ".changed" {
		t.Error("caller mutated the package suffix manifest")
	}
}

func TestFindCached(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"foo-1.0-1-x86_64.pkg.tar.zst",
		"foo-1.0-1-x86_64.pkg.tar.zst.sig",
		"foo-bar-2.0-1-x86_64.pkg.tar.zst",
		"foo-1.0-1-1-x86_64.pkg.tar.zst",
		"foo-1.0-1-x86_64.pkg.tar.zst.part",
		"epochpkg-2_1.0-1-x86_64.pkg.tar.zst",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, ok := FindCached([]string{dir}, "foo", "1.0-1", "x86_64")
	if !ok || filepath.Base(got) != "foo-1.0-1-x86_64.pkg.tar.zst" {
		t.Errorf("FindCached(foo) = %q, %t", got, ok)
	}
	got, ok = FindCached([]string{dir}, "foo", "1.0-1", "")
	if !ok || filepath.Base(got) != "foo-1.0-1-x86_64.pkg.tar.zst" {
		t.Errorf("FindCached(foo, any arch) = %q, %t", got, ok)
	}
	got, ok = FindCached([]string{dir}, "epochpkg", "2:1.0-1", "x86_64")
	if !ok || filepath.Base(got) != "epochpkg-2_1.0-1-x86_64.pkg.tar.zst" {
		t.Errorf("FindCached(epochpkg) = %q, %t", got, ok)
	}
	// A same-version file for another arch must not satisfy an x86_64 install.
	if _, ok := FindCached([]string{dir}, "foo", "1.0-1", "i686"); ok {
		t.Error("FindCached matched a foreign-arch package file")
	}
	if _, ok := FindCached([]string{dir}, "missing", "1.0-1", "x86_64"); ok {
		t.Error("FindCached found an absent package")
	}
}
