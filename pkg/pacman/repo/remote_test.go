package repo

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

// A CachyOS db built with `repo-add --use-new-db-format` carries a single SQLite
// pacman.db instead of desc entries. Parsing it must fail loudly, not silently
// return an empty repo (which a diff build would read as "rebuild everything").
func TestRemoteRepoFromDB_NewDBFormat(t *testing.T) {
	var gz bytes.Buffer
	gw := gzip.NewWriter(&gz)
	tw := tar.NewWriter(gw)
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
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := RemoteRepoFromDB("cachyos", &gz); !errors.Is(err, ErrUnsupportedDBFormat) {
		t.Fatalf("new-db-format db: want ErrUnsupportedDBFormat, got %v", err)
	}
}

func TestRepoFromURL_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := RepoFromURL(srv.URL, "alterlinux"); !errors.Is(err, ErrRepoNotFound) {
		t.Fatalf("RepoFromURL on 404: want ErrRepoNotFound, got %v", err)
	}
}

// An empty (but valid) db is the bootstrap state a freshly-initialized ayato
// repo serves: it must parse to zero packages, not error, so a diff build treats
// every local package as missing and builds them all.
func TestRepoFromURL_EmptyDB(t *testing.T) {
	var gz bytes.Buffer
	gw := gzip.NewWriter(&gz)
	if err := tar.NewWriter(gw).Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	body := gz.Bytes()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	r, err := RepoFromURL(srv.URL, "alterlinux")
	if err != nil {
		t.Fatalf("RepoFromURL on empty db: %v", err)
	}
	if len(r.Pkgs) != 0 {
		t.Fatalf("empty db: want 0 packages, got %d", len(r.Pkgs))
	}
}

func TestRemoteRepoFromDBContextHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := RemoteRepoFromDBContext(ctx, "test", strings.NewReader("unused")); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestRemoteRepoFromDBRejectsOversizedDesc(t *testing.T) {
	var gz bytes.Buffer
	gw := gzip.NewWriter(&gz)
	tw := tar.NewWriter(gw)
	body := make([]byte, maxRepoDBDescBytes+1)
	if err := tw.WriteHeader(&tar.Header{
		Name:     "foo-1-1/desc",
		Mode:     0o644,
		Size:     int64(len(body)),
		Typeflag: tar.TypeReg,
	}); err != nil {
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

	if _, err := RemoteRepoFromDB("test", &gz); !errors.Is(err, ErrRepoDBLimitExceeded) {
		t.Fatalf("error = %v, want ErrRepoDBLimitExceeded", err)
	}
}

func TestRepoDBReaderBoundsExpandedInput(t *testing.T) {
	reader := &repoDBReader{
		ctx:       context.Background(),
		reader:    strings.NewReader("abc"),
		remaining: 2,
	}
	data, err := io.ReadAll(reader)
	if !errors.Is(err, ErrRepoDBLimitExceeded) {
		t.Fatalf("error = %v, want ErrRepoDBLimitExceeded", err)
	}
	if string(data) != "ab" {
		t.Fatalf("data = %q, want %q", data, "ab")
	}
}
