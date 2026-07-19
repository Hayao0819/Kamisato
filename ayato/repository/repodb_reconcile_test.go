package repository

import (
	"bytes"
	"io"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/platform"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

func TestReconcileDBNeverImportsStaleFilesIntoCanonical(t *testing.T) {
	mem := newMemStore()
	repository := &binaryRepository{Store: mem}
	if err := repository.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	v1 := makePkg(t, dir, "foo", "1.0-1", "x86_64")
	if err := mem.StoreFile("r", "x86_64", openSeek(t, v1)); err != nil {
		t.Fatal(err)
	}
	if err := repository.RepoAdd(
		"r", "x86_64", openSeek(t, v1), nil, false, nil,
	); err != nil {
		t.Fatal(err)
	}
	staleStream, err := mem.FetchFile("r", "x86_64", "r.files.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	staleFiles, err := io.ReadAll(staleStream)
	_ = staleStream.Close()
	if err != nil {
		t.Fatal(err)
	}

	v2 := makePkg(t, dir, "foo", "2.0-1", "x86_64")
	if err := mem.StoreFile("r", "x86_64", openSeek(t, v2)); err != nil {
		t.Fatal(err)
	}
	if err := repository.RepoAdd(
		"r", "x86_64", openSeek(t, v2), nil, false, nil,
	); err != nil {
		t.Fatal(err)
	}
	beforeVersion := canonicalVersion(t, mem)
	storeStaleFiles(t, mem, staleFiles)

	if err := repository.ReconcileDB("r", "x86_64", false, nil); err != nil {
		t.Fatal(err)
	}
	remote, afterVersion := storedRepo(t, mem, "r.db.tar.gz")
	if beforeVersion != afterVersion {
		t.Fatalf("reconcile rewrote canonical: %s -> %s", beforeVersion, afterVersion)
	}
	if len(remote.Pkgs) != 1 || remote.Pkgs[0].Version() != "2.0-1" {
		t.Fatalf("canonical after reconcile = %v, want foo v2", remote.Pkgs)
	}
	filesRepo, _ := storedRepo(t, mem, "r.files.tar.gz")
	if len(filesRepo.Pkgs) != 1 || filesRepo.Pkgs[0].Version() != "2.0-1" {
		t.Fatalf("derived files after reconcile = %v, want foo v2", filesRepo.Pkgs)
	}

	if err := repository.RepoRemove("r", "x86_64", "foo", false, nil); err != nil {
		t.Fatal(err)
	}
	beforeVersion = canonicalVersion(t, mem)
	storeStaleFiles(t, mem, staleFiles)
	if err := repository.ReconcileDB("r", "x86_64", false, nil); err != nil {
		t.Fatal(err)
	}
	emptyCanonical, afterVersion := storedRepo(t, mem, "r.db.tar.gz")
	if beforeVersion != afterVersion || len(emptyCanonical.Pkgs) != 0 {
		t.Fatalf(
			"stale files resurrected package: version %s -> %s, packages=%v",
			beforeVersion,
			afterVersion,
			emptyCanonical.Pkgs,
		)
	}
	emptyFiles, _ := storedRepo(t, mem, "r.files.tar.gz")
	if len(emptyFiles.Pkgs) != 0 {
		t.Fatalf("derived files retained deleted package: %v", emptyFiles.Pkgs)
	}
}

func TestReconcileDBDiscardsCorruptDerivedFiles(t *testing.T) {
	mem := newMemStore()
	repository := &binaryRepository{Store: mem}
	if err := repository.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatal(err)
	}
	pkgPath := makePkg(t, t.TempDir(), "foo", "1.0-1", "x86_64")
	if err := mem.StoreFile("r", "x86_64", openSeek(t, pkgPath)); err != nil {
		t.Fatal(err)
	}
	if err := repository.RepoAdd(
		"r", "x86_64", openSeek(t, pkgPath), nil, false, nil,
	); err != nil {
		t.Fatal(err)
	}

	beforeVersion := canonicalVersion(t, mem)
	corrupt := platform.NewFileStream(
		"r.files.tar.gz",
		"application/octet-stream",
		nopSeekCloser{bytes.NewReader([]byte{0x1f, 0x8b, 0x08, 0x00})},
	)
	if err := mem.StoreFile("r", "x86_64", corrupt); err != nil {
		t.Fatal(err)
	}
	if err := repository.ReconcileDB("r", "x86_64", false, nil); err != nil {
		t.Fatalf("ReconcileDB with corrupt .files: %v", err)
	}

	canonical, afterVersion := storedRepo(t, mem, "r.db.tar.gz")
	if beforeVersion != afterVersion {
		t.Fatalf("derived repair rewrote canonical: %s -> %s", beforeVersion, afterVersion)
	}
	if canonical.PkgByPkgName("foo") == nil {
		t.Fatalf("canonical package disappeared: %v", canonical.Pkgs)
	}
	files, _ := storedRepo(t, mem, "r.files.tar.gz")
	if files.PkgByPkgName("foo") == nil {
		t.Fatalf("repaired .files = %v, want foo", files.Pkgs)
	}
}

func canonicalVersion(t *testing.T, mem *memStore) string {
	t.Helper()
	file, version, err := mem.FetchFileWithETag("r", "x86_64", "r.db.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	return version
}

func storeStaleFiles(t *testing.T, mem *memStore, body []byte) {
	t.Helper()
	file := platform.NewFileStream(
		"r.files.tar.gz",
		"application/octet-stream",
		nopSeekCloser{bytes.NewReader(body)},
	)
	if err := mem.StoreFile("r", "x86_64", file); err != nil {
		t.Fatal(err)
	}
}

func storedRepo(
	t *testing.T,
	mem *memStore,
	name string,
) (*repo.RemoteRepo, string) {
	t.Helper()
	file, version, err := mem.FetchFileWithETag("r", "x86_64", name)
	if err != nil {
		t.Fatal(err)
	}
	remote, err := repo.RemoteRepoFromDB("r", file)
	_ = file.Close()
	if err != nil {
		t.Fatal(err)
	}
	return remote, version
}
