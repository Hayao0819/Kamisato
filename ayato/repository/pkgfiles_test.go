package repository

import (
	"archive/tar"
	"errors"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/klauspost/compress/zstd"
)

// makePkgWithFiles builds a .pkg.tar.zst carrying a .PKGINFO plus the given
// payload members, so the native db writer records a non-empty %FILES% for it.
func makePkgWithFiles(t *testing.T, dir, name, ver, arch string, files []string) string {
	t.Helper()
	pkginfo := "pkgname = " + name + "\npkgver = " + ver + "\narch = " + arch + "\nsize = 0\n"
	out := path.Join(dir, name+"-"+ver+"-"+arch+".pkg.tar.zst")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw, err := zstd.NewWriter(f)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(zw)
	write := func(hdr *tar.Header, body []byte) {
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	write(&tar.Header{Name: ".PKGINFO", Mode: 0o644, Size: int64(len(pkginfo)), Typeflag: tar.TypeReg}, []byte(pkginfo))
	for _, fn := range files {
		write(&tar.Header{Name: fn, Mode: 0o644, Size: 0, Typeflag: tar.TypeReg}, nil)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestPkgFilesRoundTrip proves PkgFiles reads back the file list the native db
// writer recorded.
func TestPkgFilesRoundTrip(t *testing.T) {
	mem := newMemStore()
	r := &binaryRepository{Store: mem}
	if err := r.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatalf("InitArch: %v", err)
	}
	dir := t.TempDir()
	want := []string{"usr/bin/foo", "usr/share/foo/data"}
	pkgPath := makePkgWithFiles(t, dir, "foo", "1.0-1", "x86_64", want)
	if err := r.RepoAdd("r", "x86_64", openSeek(t, pkgPath), nil, false, nil); err != nil {
		t.Fatalf("RepoAdd: %v", err)
	}

	got, err := r.PkgFiles("r", "x86_64", "foo")
	if err != nil {
		t.Fatalf("PkgFiles: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PkgFiles = %v, want %v", got, want)
	}

	// A package with no entry is a not-found, not an empty list.
	if _, err := r.PkgFiles("r", "x86_64", "ghost"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("PkgFiles(ghost) error = %v, want ErrNotFound", err)
	}
}
