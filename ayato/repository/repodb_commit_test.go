package repository

import (
	"bytes"
	"io"
	"path"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/platform"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

func TestStoreFileImmutableReusesOnlyByteIdenticalObject(t *testing.T) {
	mem := newMemStore()
	repository := &binaryRepository{Store: mem}
	file := func(body string) platform.SeekFile {
		return platform.NewFileStream(
			"foo-1.0-1-x86_64.pkg.tar.zst",
			"application/octet-stream",
			nopSeekCloser{bytes.NewReader([]byte(body))},
		)
	}

	created, err := repository.StoreFileImmutable("r", "x86_64", file("winner"))
	if err != nil || !created {
		t.Fatalf("first create = (%v, %v), want true, nil", created, err)
	}
	stored, beforeVersion, err := mem.FetchFileWithETag(
		"r", "x86_64", "foo-1.0-1-x86_64.pkg.tar.zst",
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = stored.Close()
	created, err = repository.StoreFileImmutable("r", "x86_64", file("winner"))
	if err != nil || created {
		t.Fatalf("identical reuse = (%v, %v), want false, nil", created, err)
	}
	stored, afterVersion, err := mem.FetchFileWithETag(
		"r", "x86_64", "foo-1.0-1-x86_64.pkg.tar.zst",
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = stored.Close()
	if beforeVersion == afterVersion {
		t.Fatal("identical reuse did not renew the object's version")
	}
	if _, err := repository.StoreFileImmutable(
		"r", "x86_64", file("attacker"),
	); !errors.Is(err, ErrImmutableObjectConflict) {
		t.Fatalf("different-content reuse = %v, want ErrImmutableObjectConflict", err)
	}
	stored, err = mem.FetchFile("r", "x86_64", "foo-1.0-1-x86_64.pkg.tar.zst")
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(stored)
	_ = stored.Close()
	if err != nil || string(got) != "winner" {
		t.Fatalf("immutable object = %q, %v; want winner", got, err)
	}
}

func TestRepoAddConditionalRejectsConcurrentDowngrade(t *testing.T) {
	mem := newMemStore()
	repository := &binaryRepository{Store: mem}
	if err := repository.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	oldPath := makePkg(t, dir, "foo", "0.9-1", "x86_64")
	if err := repository.RepoAdd(
		"r", "x86_64", openSeek(t, oldPath), nil, false, nil,
	); err != nil {
		t.Fatal(err)
	}
	conditional := func(pkg platform.SeekFile) error {
		return repository.RepoAddBatch("r", "x86_64", []RepoAddItem{{
			Pkg:                    pkg,
			CheckCurrent:           true,
			ExpectedName:           "foo",
			ExpectedCurrentVersion: "0.9-1",
			ExpectedCurrentFile:    path.Base(oldPath),
		}}, false, nil)
	}
	if err := conditional(openSeek(
		t, makePkg(t, dir, "foo", "2.0-1", "x86_64"),
	)); err != nil {
		t.Fatal(err)
	}
	err := conditional(openSeek(t, makePkg(t, dir, "foo", "1.0-1", "x86_64")))
	if !errors.Is(err, ErrPackageChanged) {
		t.Fatalf("stale publish = %v, want ErrPackageChanged", err)
	}
	remote, err := repository.RemoteRepo("r", "x86_64")
	if err != nil {
		t.Fatal(err)
	}
	if got := remote.PkgByPkgName("foo").Version(); got != "2.0-1" {
		t.Fatalf("stale writer downgraded package to %s", got)
	}
}

func TestRepoAddReportsCanonicalCommitBeforeDerivedFailure(t *testing.T) {
	mem := newMemStore()
	repository := &binaryRepository{Store: mem}
	if err := repository.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("files write failed")
	mem.storeErrName = "r.files.tar.gz"
	mem.storeErr = boom
	pkgPath := makePkg(t, t.TempDir(), "foo", "1.0-1", "x86_64")
	if err := mem.StoreFile("r", "x86_64", openSeek(t, pkgPath)); err != nil {
		t.Fatal(err)
	}
	err := repository.RepoAdd(
		"r", "x86_64", openSeek(t, pkgPath), nil, false, nil,
	)
	if !CanonicalCommitted(err) || !errors.Is(err, boom) {
		t.Fatalf("RepoAdd = %v, want committed wrapper around boom", err)
	}

	mem.storeErr = nil
	remote, err := repository.RemoteRepo("r", "x86_64")
	if err != nil || remote.PkgByPkgName("foo") == nil {
		t.Fatalf("canonical DB = %v, %v; want foo", remote, err)
	}
	if err := repository.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatalf("startup reconciliation: %v", err)
	}
	files, err := mem.FetchFile("r", "x86_64", "r.files.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	filesRepo, err := repo.RemoteRepoFromDB("r", files)
	_ = files.Close()
	if err != nil || filesRepo.PkgByPkgName("foo") == nil {
		t.Fatalf("reconciled files DB = %v, %v; want foo", filesRepo, err)
	}
}

func TestRepoAddTreatsCanonicalWriteResponseFailureAsCommitted(t *testing.T) {
	mem := newMemStore()
	repository := &binaryRepository{Store: mem}
	if err := repository.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("response lost after canonical write")
	mem.storeAfterErrName = "r.db.tar.gz"
	mem.storeAfterErr = boom
	pkgPath := makePkg(t, t.TempDir(), "foo", "1.0-1", "x86_64")
	err := repository.RepoAdd(
		"r", "x86_64", openSeek(t, pkgPath), nil, false, nil,
	)
	if !CanonicalCommitted(err) || !errors.Is(err, boom) {
		t.Fatalf("RepoAdd = %v, want committed ambiguous outcome", err)
	}
	remote, fetchErr := repository.RemoteRepo("r", "x86_64")
	if fetchErr != nil || remote.PkgByPkgName("foo") == nil {
		t.Fatalf("canonical foo = %v, %v; want visible", remote, fetchErr)
	}
}
