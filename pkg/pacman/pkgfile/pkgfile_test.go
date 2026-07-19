package pkgfile

import (
	"errors"
	"testing"
)

func TestParse(t *testing.T) {
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(tt.filename)
			wantOK := tt.wantArchive || tt.wantSig
			if (err == nil) != wantOK {
				t.Fatalf("Parse(%q) error = %v, want success %t", tt.filename, err, wantOK)
			}
			if !wantOK {
				if !errors.Is(err, ErrInvalid) {
					t.Fatalf("Parse(%q) error = %v, want ErrInvalid", tt.filename, err)
				}
				return
			}
			if got.IsSignature() != tt.wantSig {
				t.Errorf("IsSignature = %t, want %t", got.IsSignature(), tt.wantSig)
			}
			if IsArchive(tt.filename) != tt.wantArchive {
				t.Errorf("IsArchive(%q) = %t, want %t", tt.filename, IsArchive(tt.filename), tt.wantArchive)
			}
			if !IsArtifact(tt.filename) {
				t.Errorf("IsArtifact(%q) = false", tt.filename)
			}
		})
	}
}

func TestCoordinates(t *testing.T) {
	t.Parallel()
	file, err := Parse("python-foo-2:1.2.3-4-any.pkg.tar.zst.sig")
	if err != nil {
		t.Fatal(err)
	}
	coords, err := file.Coordinates()
	if err != nil {
		t.Fatal(err)
	}
	if coords.Name != "python-foo" || coords.Version != "2:1.2.3" ||
		coords.Release != "4" || coords.Arch != "any" {
		t.Fatalf("coordinates = %#v", coords)
	}
	if !coords.IsAny() {
		t.Error("IsAny = false")
	}
	if !IsAny(file.Filename()) {
		t.Error("IsAny(filename) = false")
	}
	if !coords.MatchesMetadata("python-foo", "2:1.2.3-4", "any") {
		t.Error("coordinates do not match equivalent PKGINFO")
	}
	if file.ArchiveFilename() != "python-foo-2:1.2.3-4-any.pkg.tar.zst" {
		t.Errorf("ArchiveFilename = %q", file.ArchiveFilename())
	}
	if file.SignatureFilename() != file.Filename() {
		t.Errorf("SignatureFilename = %q, want %q", file.SignatureFilename(), file.Filename())
	}

	legacy, err := Parse("python-foo-2_1.2.3-4-any.pkg.tar.zst")
	if err != nil {
		t.Fatal(err)
	}
	legacyCoords, err := legacy.Coordinates()
	if err != nil || !legacyCoords.MatchesMetadata("python-foo", "2:1.2.3-4", "any") {
		t.Errorf("legacy underscore epoch no longer matches metadata: coordinates=%+v error=%v", legacyCoords, err)
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
		file, err := Parse(name)
		if err != nil {
			t.Fatalf("Parse(%q): %v", name, err)
		}
		if _, err := file.Coordinates(); !errors.Is(err, ErrInvalid) {
			t.Errorf("Coordinates(%q) error = %v, want ErrInvalid", name, err)
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
	second := SupportedArchiveSuffixes()
	if second[0] == ".changed" {
		t.Error("caller mutated the package suffix manifest")
	}
}
