package localfs_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob/localfs"
)

func TestLocalStoreOrphanDeleteKeepsRenewedObject(t *testing.T) {
	root := t.TempDir()
	store := localfs.New(root, []string{"myrepo"})
	payload := []byte("payload")
	if err := store.StoreFile("myrepo", "x86_64", seekFile(payload)); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(filepath.Join(root, "myrepo", "x86_64", testPackageName), old, old); err != nil {
		t.Fatal(err)
	}
	infos, err := store.FilesWithMeta("myrepo", "x86_64")
	if err != nil || len(infos) != 1 {
		t.Fatalf("FilesWithMeta = %v, %v", infos, err)
	}
	file, etag, err := store.FetchFileWithETag("myrepo", "x86_64", testPackageName)
	if err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	if err := store.StoreFileIfMatch(
		"myrepo", "x86_64", seekFile(payload), etag,
	); err != nil {
		t.Fatalf("renew immutable object: %v", err)
	}
	deleted, err := store.DeleteFileIfUnchanged(
		"myrepo", "x86_64", infos[0], time.Now().Add(-time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	if deleted {
		t.Fatal("collector deleted an object renewed by a concurrent publisher")
	}
	if file, err := store.FetchFile("myrepo", "x86_64", testPackageName); err != nil {
		t.Fatalf("renewed object missing: %v", err)
	} else {
		_ = file.Close()
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
		t.Fatal("second writer acquired the repo lock before the first released it")
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
		t.Fatal("second writer did not acquire the repo lock after release")
	}
}
