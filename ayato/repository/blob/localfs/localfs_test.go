package localfs_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob/localfs"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

// nopSeekCloser adapts an in-memory reader into the io.ReadSeekCloser that
// stream.NewFileStream needs, so a test can build a SeekFile without a real file.
type nopSeekCloser struct{ *bytes.Reader }

func (nopSeekCloser) Close() error { return nil }

func seekFile(name string, data []byte) stream.SeekFile {
	return stream.NewFileStream(name, "application/octet-stream", nopSeekCloser{bytes.NewReader(data)})
}

func TestLocalStorePutGetDelete(t *testing.T) {
	store := localfs.New(t.TempDir(), []string{"myrepo"})
	const name = "pkg-1.0-1-x86_64.pkg.tar.zst"
	want := []byte("payload")

	if err := store.StoreFile("myrepo", "x86_64", seekFile(name, want)); err != nil {
		t.Fatalf("StoreFile: %v", err)
	}

	f, err := store.FetchFile("myrepo", "x86_64", name)
	if err != nil {
		t.Fatalf("FetchFile: %v", err)
	}
	got, _ := io.ReadAll(f)
	_ = f.Close()
	if !bytes.Equal(got, want) {
		t.Fatalf("FetchFile content = %q, want %q", got, want)
	}

	if err := store.DeleteFile("myrepo", "x86_64", name); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	if _, err := store.FetchFile("myrepo", "x86_64", name); !errors.Is(err, blob.ErrNotFound) {
		t.Fatalf("FetchFile after delete = %v, want ErrNotFound", err)
	}
}

func TestLocalStoreCompareAndSwap(t *testing.T) {
	store := localfs.New(t.TempDir(), []string{"myrepo"})
	const name = "pkg-1.0-1-x86_64.pkg.tar.zst"
	if err := store.StoreFileIfMatch("myrepo", "x86_64", seekFile(name, []byte("v1")), ""); err != nil {
		t.Fatalf("create-only write: %v", err)
	}
	if err := store.StoreFileIfMatch("myrepo", "x86_64", seekFile(name, []byte("clobber")), ""); !errors.Is(err, blob.ErrPreconditionFailed) {
		t.Fatalf("second create-only write = %v, want ErrPreconditionFailed", err)
	}
	f, etag, err := store.FetchFileWithETag("myrepo", "x86_64", name)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if etag == "" {
		t.Fatal("localfs must return a strong ETag")
	}
	if err := store.StoreFileIfMatch("myrepo", "x86_64", seekFile(name, []byte("v2")), etag); err != nil {
		t.Fatalf("matching update: %v", err)
	}
	if err := store.StoreFileIfMatch("myrepo", "x86_64", seekFile(name, []byte("stale")), etag); !errors.Is(err, blob.ErrPreconditionFailed) {
		t.Fatalf("stale update = %v, want ErrPreconditionFailed", err)
	}
	f, err = store.FetchFile("myrepo", "x86_64", name)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(f)
	_ = f.Close()
	if err != nil || string(body) != "v2" {
		t.Fatalf("CAS content = %q, %v; want v2", body, err)
	}
}

func TestLocalStoreFetchMissingIsErrNotFound(t *testing.T) {
	store := localfs.New(t.TempDir(), []string{"myrepo"})
	if _, err := store.FetchFile("myrepo", "x86_64", "absent.pkg.tar.zst"); !errors.Is(err, blob.ErrNotFound) {
		t.Fatalf("FetchFile(absent) = %v, want ErrNotFound", err)
	}
}

func TestLocalStoreRejectsTraversalAndUnknownRepo(t *testing.T) {
	store := localfs.New(t.TempDir(), []string{"myrepo"})

	// A repo outside the allowlist is refused before any disk access.
	if _, err := store.FetchFile("evil", "x86_64", "p"); err == nil {
		t.Error("FetchFile allowed a repo outside the allowlist")
	}
	// A "../" or separator in the arch or file component is refused too.
	for _, tc := range []struct{ arch, file string }{
		{"..", "p"},
		{"x86_64", "../etc/passwd"},
		{"x86_64", "a/b"},
		{"", "p"},
		{"x86_64", ""},
	} {
		if _, err := store.FetchFile("myrepo", tc.arch, tc.file); err == nil {
			t.Errorf("FetchFile(myrepo, %q, %q) = nil, want error", tc.arch, tc.file)
		}
	}
	if _, err := store.Files("myrepo", ".."); err == nil {
		t.Error("Files allowed an unsafe architecture")
	}
	if _, err := store.FilesWithMeta("myrepo", "../other"); err == nil {
		t.Error("FilesWithMeta allowed an unsafe architecture")
	}

	// Even an invalid name accidentally present in a configured allowlist cannot
	// turn path.Join(root, repo) into a traversal.
	unsafeAllowlist := localfs.New(t.TempDir(), []string{".."})
	if _, err := unsafeAllowlist.FetchFile("..", "x86_64", "p"); err == nil {
		t.Error("FetchFile allowed an unsafe repository name from its allowlist")
	}
}

func TestLocalStoreFilesWithMeta(t *testing.T) {
	store := localfs.New(t.TempDir(), []string{"myrepo"})
	const name = "pkg-1.0-1-x86_64.pkg.tar.zst"
	if err := store.StoreFile("myrepo", "x86_64", seekFile(name, []byte("payload"))); err != nil {
		t.Fatalf("StoreFile: %v", err)
	}

	infos, err := store.FilesWithMeta("myrepo", "x86_64")
	if err != nil {
		t.Fatalf("FilesWithMeta: %v", err)
	}
	if len(infos) != 1 || infos[0].Name != name {
		t.Fatalf("FilesWithMeta = %v, want one entry named %q", infos, name)
	}
	if infos[0].LastModified.IsZero() {
		t.Errorf("FilesWithMeta ModTime is zero, want the file's on-disk mtime")
	}
	if time.Since(infos[0].LastModified) > time.Hour {
		t.Errorf("FilesWithMeta ModTime %v is implausibly old", infos[0].LastModified)
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

func TestLocalStoreOrphanDeleteKeepsRenewedObject(t *testing.T) {
	root := t.TempDir()
	store := localfs.New(root, []string{"myrepo"})
	const name = "pkg-1.0-1-x86_64.pkg.tar.zst"
	payload := []byte("payload")
	if err := store.StoreFile("myrepo", "x86_64", seekFile(name, payload)); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(filepath.Join(root, "myrepo", "x86_64", name), old, old); err != nil {
		t.Fatal(err)
	}
	infos, err := store.FilesWithMeta("myrepo", "x86_64")
	if err != nil || len(infos) != 1 {
		t.Fatalf("FilesWithMeta = %v, %v", infos, err)
	}
	f, etag, err := store.FetchFileWithETag("myrepo", "x86_64", name)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if err := store.StoreFileIfMatch("myrepo", "x86_64", seekFile(name, payload), etag); err != nil {
		t.Fatalf("renew immutable object: %v", err)
	}
	deleted, err := store.DeleteFileIfUnchanged("myrepo", "x86_64", infos[0], time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if deleted {
		t.Fatal("collector deleted an object renewed by a concurrent publisher")
	}
	if f, err := store.FetchFile("myrepo", "x86_64", name); err != nil {
		t.Fatalf("renewed object missing: %v", err)
	} else {
		_ = f.Close()
	}
}

func TestLocalStorePublicationLockSerializesRepositoryWriters(t *testing.T) {
	root := t.TempDir()
	first := localfs.New(root, []string{"myrepo"})
	second := localfs.New(root, []string{"myrepo"})

	releaseFirst, err := first.LockPublication("myrepo")
	if err != nil {
		t.Fatalf("first LockPublication: %v", err)
	}
	defer releaseFirst()

	started := make(chan struct{})
	acquired := make(chan func(), 1)
	errCh := make(chan error, 1)
	go func() {
		close(started)
		release, err := second.LockPublication("myrepo")
		if err != nil {
			errCh <- err
			return
		}
		acquired <- release
	}()
	<-started

	select {
	case release := <-acquired:
		release()
		t.Fatal("second publication writer acquired the repo lock before the first released it")
	case err := <-errCh:
		t.Fatalf("second LockPublication: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	releaseFirst()
	select {
	case release := <-acquired:
		release()
	case err := <-errCh:
		t.Fatalf("second LockPublication after release: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("second publication writer did not acquire the repo lock after release")
	}
}
