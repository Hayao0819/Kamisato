package repository

import (
	"sync"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

func pauseCanonicalCommit(mem *memStore) (<-chan struct{}, func()) {
	reached := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	mem.mu.Lock()
	mem.afterStoreName = "r.db.tar.gz"
	mem.afterStore = func(string) {
		close(reached)
		<-release
	}
	mem.mu.Unlock()
	return reached, func() { once.Do(func() { close(release) }) }
}

func waitForSignal(t *testing.T, signal <-chan struct{}, message string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatal(message)
	}
}

func waitForResult(t *testing.T, result <-chan error, message string) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal(message)
		return nil
	}
}

func TestConcurrentWritersReconcileDerivedArtifactsToCanonicalDB(t *testing.T) {
	mem := newMemStore()
	first := &binaryRepository{Store: mem}
	second := &binaryRepository{Store: mem}
	if err := first.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatal(err)
	}

	canonicalCommitted, release := pauseCanonicalCommit(mem)
	defer release()
	dir := t.TempDir()
	alpha := openSeek(t, makePkg(t, dir, "alpha", "1.0-1"))
	bravo := openSeek(t, makePkg(t, dir, "bravo", "1.0-1"))
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- first.RepoAddBatch("r", "x86_64", []RepoAddItem{{
			Pkg:             alpha,
			CheckCurrent:    true,
			ExpectedName:    "alpha",
			IntendedVersion: "1.0-1",
			IntendedFile:    "alpha-1.0-1-x86_64.pkg.tar.zst",
		}}, false, nil)
	}()
	waitForSignal(t, canonicalCommitted, "first writer did not commit canonical DB")

	secondDone := make(chan error, 1)
	go func() {
		secondDone <- second.RepoAdd("r", "x86_64", bravo, nil, false, nil)
	}()
	if err := waitForResult(
		t, secondDone, "second writer did not complete while first was paused",
	); err != nil {
		release()
		t.Fatal(err)
	}
	release()
	if err := waitForResult(
		t, firstDone, "first writer did not reconcile after conflict",
	); err != nil {
		t.Fatal(err)
	}

	for _, artifact := range []string{"r.db.tar.gz", "r.files.tar.gz"} {
		file, err := mem.FetchFile("r", "x86_64", artifact)
		if err != nil {
			t.Fatalf("fetch %s: %v", artifact, err)
		}
		remote, err := repo.RemoteRepoFromDB("r", file)
		_ = file.Close()
		if err != nil {
			t.Fatalf("parse %s: %v", artifact, err)
		}
		for _, name := range []string{"alpha", "bravo"} {
			if remote.PkgByPkgName(name) == nil {
				t.Fatalf("%s is stale: missing %s", artifact, name)
			}
		}
	}
}

func TestPostCanonicalSupersessionReturnsCommittedOutcome(t *testing.T) {
	mem := newMemStore()
	first := &binaryRepository{Store: mem}
	second := &binaryRepository{Store: mem}
	if err := first.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatal(err)
	}

	canonicalCommitted, release := pauseCanonicalCommit(mem)
	defer release()
	dir := t.TempDir()
	alpha1 := openSeek(t, makePkg(t, dir, "alpha", "1.0-1"))
	charlie1 := openSeek(t, makePkg(t, dir, "charlie", "1.0-1"))
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- first.RepoAddBatch("r", "x86_64", []RepoAddItem{
			{
				Pkg:          alpha1,
				CheckCurrent: true, ExpectedName: "alpha",
				IntendedVersion: "1.0-1",
				IntendedFile:    "alpha-1.0-1-x86_64.pkg.tar.zst",
			},
			{
				Pkg:          charlie1,
				CheckCurrent: true, ExpectedName: "charlie",
				IntendedVersion: "1.0-1",
				IntendedFile:    "charlie-1.0-1-x86_64.pkg.tar.zst",
			},
		}, false, nil)
	}()
	waitForSignal(t, canonicalCommitted, "first writer did not commit canonical DB")

	alpha2 := openSeek(t, makePkg(t, dir, "alpha", "2.0-1"))
	err := second.RepoAddBatch("r", "x86_64", []RepoAddItem{{
		Pkg:                    alpha2,
		CheckCurrent:           true,
		ExpectedName:           "alpha",
		ExpectedCurrentVersion: "1.0-1",
		ExpectedCurrentFile:    "alpha-1.0-1-x86_64.pkg.tar.zst",
		IntendedVersion:        "2.0-1",
		IntendedFile:           "alpha-2.0-1-x86_64.pkg.tar.zst",
	}}, false, nil)
	if err != nil {
		release()
		t.Fatal(err)
	}
	release()
	err = waitForResult(t, firstDone, "first writer did not finish after supersession")
	if !CanonicalCommitted(err) || !errors.Is(err, ErrPackageChanged) {
		t.Fatalf("first writer = %v, want committed ErrPackageChanged", err)
	}

	remote, err := first.RemoteRepo("r", "x86_64")
	if err != nil {
		t.Fatal(err)
	}
	if alpha := remote.PkgByPkgName("alpha"); alpha == nil || alpha.Version() != "2.0-1" {
		t.Fatalf("newer alpha was not retained: %v", alpha)
	}
	if remote.PkgByPkgName("charlie") == nil {
		t.Fatal("second canonical base should include first writer's other package")
	}
}
