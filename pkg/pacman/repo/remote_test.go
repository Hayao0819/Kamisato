package repo

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"
)

func TestRemoteRepoFromDB(t *testing.T) {
	desc := "%FILENAME%\nfoo-1.0-1-x86_64.pkg.tar.zst\n\n" +
		"%NAME%\nfoo\n\n%BASE%\nfoobase\n\n%VERSION%\n1.0-1\n\n%ARCH%\nx86_64\n"

	var gz bytes.Buffer
	gw := gzip.NewWriter(&gz)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{Name: "foo-1.0-1/", Mode: 0o755, Typeflag: tar.TypeDir}); err != nil {
		t.Fatal(err)
	}
	body := []byte(desc)
	if err := tw.WriteHeader(&tar.Header{Name: "foo-1.0-1/desc", Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := RemoteRepoFromDB("test", &gz)
	if err != nil {
		t.Fatalf("RemoteRepoFromDB: %v", err)
	}
	if len(r.Pkgs) != 1 {
		t.Fatalf("want 1 package, got %d", len(r.Pkgs))
	}

	p := r.PkgByPkgName("foo")
	if p == nil {
		t.Fatal("PkgByPkgName(foo) = nil")
	}
	if got := p.Path(); got != "foo-1.0-1-x86_64.pkg.tar.zst" {
		t.Errorf("Path() = %q, want the desc FILENAME", got)
	}
	if r.PkgByPkgBase("foobase") == nil {
		t.Error("PkgByPkgBase(foobase) = nil")
	}
}
