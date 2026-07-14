package localfs_test

import (
	"bytes"
	"io"
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
