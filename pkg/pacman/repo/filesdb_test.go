package repo

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"reflect"
	"testing"
)

// writeFilesDB assembles a known ".files" archive: for each package a desc member
// carrying %NAME% and a files member carrying %FILES%, exactly as the db writer
// emits them.
func writeFilesDB(t *testing.T, pkgs map[string][]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	write := func(name string, data string) {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	for name, files := range pkgs {
		dir := name + "-1.0-1"
		write(dir+"/desc", "%NAME%\n"+name+"\n")
		var fb bytes.Buffer
		fb.WriteString("%FILES%\n")
		for _, f := range files {
			fb.WriteString(f)
			fb.WriteByte('\n')
		}
		write(dir+"/files", fb.String())
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestFilesFromDB(t *testing.T) {
	archive := writeFilesDB(t, map[string][]string{
		"foo": {"usr/bin/foo", "usr/share/foo/data"},
		"bar": {"usr/lib/libbar.so"},
	})
	byName, err := FilesFromDB(bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("FilesFromDB: %v", err)
	}
	want := []string{"usr/bin/foo", "usr/share/foo/data"}
	if got := byName["foo"]; !reflect.DeepEqual(got, want) {
		t.Errorf("foo files = %v, want %v", got, want)
	}
	if got := byName["bar"]; !reflect.DeepEqual(got, []string{"usr/lib/libbar.so"}) {
		t.Errorf("bar files = %v, want [usr/lib/libbar.so]", got)
	}
}

func TestFilesFromDB_NewDBFormat(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := append([]byte("SQLite format 3\x00"), make([]byte, 100)...)
	if err := tw.WriteHeader(&tar.Header{Name: "pacman.db", Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := FilesFromDB(&buf); !errors.Is(err, ErrUnsupportedDBFormat) {
		t.Fatalf("new-db-format files db: want ErrUnsupportedDBFormat, got %v", err)
	}
}
