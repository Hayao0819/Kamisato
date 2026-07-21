package localfs_test

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/platform"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob/localfs"
)

type nopSeekCloser struct{ *bytes.Reader }

func (nopSeekCloser) Close() error { return nil }

const testPackageName = "pkg-1.0-1-x86_64.pkg.tar.zst"

func seekFile(data []byte) platform.SeekFile {
	return platform.NewFileStream(
		testPackageName,
		"application/octet-stream",
		nopSeekCloser{bytes.NewReader(data)},
	)
}

func TestLocalStorePutGetDelete(t *testing.T) {
	store := localfs.New(t.TempDir(), []string{"myrepo"})
	want := []byte("payload")

	if err := store.StoreFile("myrepo", "x86_64", seekFile(want)); err != nil {
		t.Fatalf("StoreFile: %v", err)
	}
	file, err := store.FetchFile("myrepo", "x86_64", testPackageName)
	if err != nil {
		t.Fatalf("FetchFile: %v", err)
	}
	got, _ := io.ReadAll(file)
	_ = file.Close()
	if !bytes.Equal(got, want) {
		t.Fatalf("FetchFile content = %q, want %q", got, want)
	}

	if err := store.DeleteFile("myrepo", "x86_64", testPackageName); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	if _, err := store.FetchFile("myrepo", "x86_64", testPackageName); !errors.Is(err, blob.ErrNotFound) {
		t.Fatalf("FetchFile after delete = %v, want ErrNotFound", err)
	}
}

func TestLocalStoreCompareAndSwap(t *testing.T) {
	store := localfs.New(t.TempDir(), []string{"myrepo"})
	if err := store.StoreFileIfMatch(
		"myrepo", "x86_64", seekFile([]byte("v1")), "",
	); err != nil {
		t.Fatalf("create-only write: %v", err)
	}
	err := store.StoreFileIfMatch(
		"myrepo", "x86_64", seekFile([]byte("clobber")), "",
	)
	if !errors.Is(err, blob.ErrPreconditionFailed) {
		t.Fatalf("second create-only write = %v, want ErrPreconditionFailed", err)
	}
	file, etag, err := store.FetchFileWithETag("myrepo", "x86_64", testPackageName)
	if err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	if etag == "" {
		t.Fatal("localfs must return a strong ETag")
	}
	if err := store.StoreFileIfMatch(
		"myrepo", "x86_64", seekFile([]byte("v2")), etag,
	); err != nil {
		t.Fatalf("matching update: %v", err)
	}
	err = store.StoreFileIfMatch(
		"myrepo", "x86_64", seekFile([]byte("stale")), etag,
	)
	if !errors.Is(err, blob.ErrPreconditionFailed) {
		t.Fatalf("stale update = %v, want ErrPreconditionFailed", err)
	}
	file, err = store.FetchFile("myrepo", "x86_64", testPackageName)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(file)
	_ = file.Close()
	if err != nil || string(body) != "v2" {
		t.Fatalf("CAS content = %q, %v; want v2", body, err)
	}
}

func TestLocalStoreFetchMissingIsErrNotFound(t *testing.T) {
	store := localfs.New(t.TempDir(), []string{"myrepo"})
	if _, err := store.FetchFile(
		"myrepo", "x86_64", "absent.pkg.tar.zst",
	); !errors.Is(err, blob.ErrNotFound) {
		t.Fatalf("FetchFile(absent) = %v, want ErrNotFound", err)
	}
}

func TestLocalStoreRejectsTraversalAndUnknownRepo(t *testing.T) {
	store := localfs.New(t.TempDir(), []string{"myrepo"})
	if _, err := store.FetchFile("evil", "x86_64", "p"); err == nil {
		t.Error("FetchFile allowed a repo outside the allowlist")
	}
	for _, test := range []struct {
		arch string
		file string
	}{
		{arch: "..", file: "p"},
		{arch: "x86_64", file: "../etc/passwd"},
		{arch: "x86_64", file: "a/b"},
		{file: "p"},
		{arch: "x86_64"},
	} {
		if _, err := store.FetchFile("myrepo", test.arch, test.file); err == nil {
			t.Errorf("FetchFile(myrepo, %q, %q) = nil, want error", test.arch, test.file)
		}
	}
	if _, err := store.Files("myrepo", ".."); err == nil {
		t.Error("Files allowed an unsafe architecture")
	}
	if _, err := store.FilesWithMeta("myrepo", "../other"); err == nil {
		t.Error("FilesWithMeta allowed an unsafe architecture")
	}

	unsafeAllowlist := localfs.New(t.TempDir(), []string{".."})
	if _, err := unsafeAllowlist.FetchFile("..", "x86_64", "p"); err == nil {
		t.Error("FetchFile allowed an unsafe repository name from its allowlist")
	}
}

func TestLocalStoreFilesWithMeta(t *testing.T) {
	store := localfs.New(t.TempDir(), []string{"myrepo"})
	if err := store.StoreFile("myrepo", "x86_64", seekFile([]byte("payload"))); err != nil {
		t.Fatalf("StoreFile: %v", err)
	}

	infos, err := store.FilesWithMeta("myrepo", "x86_64")
	if err != nil {
		t.Fatalf("FilesWithMeta: %v", err)
	}
	if len(infos) != 1 || infos[0].Name != testPackageName {
		t.Fatalf("FilesWithMeta = %v, want one entry named %q", infos, testPackageName)
	}
	if infos[0].LastModified.IsZero() || time.Since(infos[0].LastModified) > time.Hour {
		t.Errorf("FilesWithMeta ModTime %v is invalid", infos[0].LastModified)
	}
}

func TestLocalStoreFilesWithMetaMissingArch(t *testing.T) {
	store := localfs.New(t.TempDir(), []string{"myrepo"})
	infos, err := store.FilesWithMeta("myrepo", "x86_64")
	if err != nil {
		t.Fatalf("FilesWithMeta(missing arch): %v", err)
	}
	if len(infos) != 0 {
		t.Errorf("FilesWithMeta(missing arch) = %v, want empty", infos)
	}
}
