package pkg

import (
	"archive/tar"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The committed fixtures under testdata are tiny shared objects linked with an
// explicit -Wl,-soname, so DT_SONAME is present and the scan is deterministic
// without needing a compiler at test time.
const (
	fixtureSO1 = "testdata/libbumptest.so.1"
	fixtureSO2 = "testdata/libbumptest.so.2"
)

func TestSonamesOfFile(t *testing.T) {
	got, err := SonamesOf(fixtureSO1)
	if err != nil {
		t.Fatalf("SonamesOf: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"libbumptest.so.1"}) {
		t.Errorf("got %q, want [libbumptest.so.1]", got)
	}
}

func TestSonamesOfArchive(t *testing.T) {
	lib, err := os.ReadFile(fixtureSO1)
	if err != nil {
		t.Fatal(err)
	}

	archive := filepath.Join(t.TempDir(), "pkg.tar")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(f)
	write := func(name string, mode int64, body []byte) {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	// A real .so, a non-.so regular file (must be ignored), and a symlink that
	// shares the .so name (must be skipped: it is not a regular ELF).
	write("usr/lib/libbumptest.so.1", 0o755, lib)
	write(".PKGINFO", 0o644, []byte("pkgname = bumptest\n"))
	if err := tw.WriteHeader(&tar.Header{Name: "usr/lib/libbumptest.so", Linkname: "libbumptest.so.1", Typeflag: tar.TypeSymlink}); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := SonamesOf(archive)
	if err != nil {
		t.Fatalf("SonamesOf: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"libbumptest.so.1"}) {
		t.Errorf("got %q, want [libbumptest.so.1]", got)
	}
}

func TestDetectBumps(t *testing.T) {
	cases := []struct {
		name     string
		old, new []string
		want     []SonameBump
	}{
		{
			name: "major bump",
			old:  []string{"libfoo.so.1"},
			new:  []string{"libfoo.so.2"},
			want: []SonameBump{{Base: "libfoo.so", Old: "libfoo.so.1", New: "libfoo.so.2"}},
		},
		{
			name: "unchanged",
			old:  []string{"libfoo.so.1"},
			new:  []string{"libfoo.so.1"},
			want: nil,
		},
		{
			name: "removed",
			old:  []string{"libfoo.so.1"},
			new:  []string{},
			want: []SonameBump{{Base: "libfoo.so", Old: "libfoo.so.1", New: ""}},
		},
		{
			name: "added is not a bump",
			old:  []string{},
			new:  []string{"libnew.so.1"},
			want: nil,
		},
		{
			name: "mixed: one bumps, one stable",
			old:  []string{"libfoo.so.1", "libbar.so.3"},
			new:  []string{"libfoo.so.2", "libbar.so.3"},
			want: []SonameBump{{Base: "libfoo.so", Old: "libfoo.so.1", New: "libfoo.so.2"}},
		},
		{
			name: "dotted name keeps its version boundary",
			old:  []string{"libpython3.11.so.1.0"},
			new:  []string{"libpython3.11.so.1.0"},
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectBumps(tc.old, tc.new)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("DetectBumps() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestSonameBase(t *testing.T) {
	cases := map[string]string{
		"libfoo.so.1":          "libfoo.so",
		"libfoo.so.1.2.3":      "libfoo.so",
		"libfoo.so":            "libfoo.so",
		"libpython3.11.so.1.0": "libpython3.11.so",
	}
	for in, want := range cases {
		if got := sonameBase(in); got != want {
			t.Errorf("sonameBase(%q) = %q, want %q", in, got, want)
		}
	}
}
