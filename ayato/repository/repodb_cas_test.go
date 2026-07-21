package repository

import (
	"sort"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

func TestRepoDBCASNoLostUpdate(t *testing.T) {
	mem := newMemStore()
	first := &binaryRepository{Store: mem}
	second := &binaryRepository{Store: mem}
	if err := first.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatalf("InitArch: %v", err)
	}
	dir := t.TempDir()
	addPackage := func(repository *binaryRepository, name string) {
		t.Helper()
		path := makePkg(t, dir, name, "1.0-1")
		if err := repository.RepoAdd(
			"r", "x86_64", openSeek(t, path), nil, false, nil,
		); err != nil {
			t.Fatalf("RepoAdd %s: %v", name, err)
		}
	}
	addPackage(first, "alpha")
	mem.onFetch = func(name string) {
		if name == "r.db.tar.gz" {
			addPackage(second, "bravo")
		}
	}
	addPackage(first, "charlie")

	remote, err := first.RemoteRepo("r", "x86_64")
	if err != nil {
		t.Fatalf("RemoteRepo: %v", err)
	}
	got := map[string]bool{}
	for _, pkg := range remote.Pkgs {
		got[pkg.Name()] = true
	}
	for _, name := range []string{"alpha", "bravo", "charlie"} {
		if !got[name] {
			t.Errorf("package %q lost; db = %v", name, sortedKeys(got))
		}
	}
}

func TestInitArchPreservesPopulatedDB(t *testing.T) {
	mem := newMemStore()
	repository := &binaryRepository{Store: mem}
	if err := repository.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatalf("InitArch (create): %v", err)
	}
	pkg := makePkg(t, t.TempDir(), "foo", "1.0-1")
	if err := repository.RepoAdd(
		"r", "x86_64", openSeek(t, pkg), nil, false, nil,
	); err != nil {
		t.Fatalf("RepoAdd: %v", err)
	}
	if err := repository.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatalf("InitArch (re-init): %v", err)
	}
	remote, err := repository.RemoteRepo("r", "x86_64")
	if err != nil {
		t.Fatalf("RemoteRepo: %v", err)
	}
	if len(remote.Pkgs) != 1 || remote.Pkgs[0].Name() != "foo" {
		t.Fatalf("re-init wiped the populated db: %v", remote.Pkgs)
	}
}

func TestRepoRemoveIdempotent(t *testing.T) {
	mem := newMemStore()
	repository := &binaryRepository{Store: mem}
	if err := repository.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatalf("InitArch: %v", err)
	}
	pkg := makePkg(t, t.TempDir(), "foo", "1.0-1")
	if err := repository.RepoAdd(
		"r", "x86_64", openSeek(t, pkg), nil, false, nil,
	); err != nil {
		t.Fatalf("RepoAdd: %v", err)
	}
	for _, name := range []string{"foo", "foo", "ghost"} {
		if err := repository.RepoRemove("r", "x86_64", name, false, nil); err != nil {
			t.Fatalf("RepoRemove(%s): %v", name, err)
		}
	}
}

func TestRepoAddSurfacesTransientFetchError(t *testing.T) {
	mem := newMemStore()
	repository := &binaryRepository{Store: mem}
	if err := repository.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	foo := makePkg(t, dir, "foo", "1.0-1")
	if err := repository.RepoAdd(
		"r", "x86_64", openSeek(t, foo), nil, false, nil,
	); err != nil {
		t.Fatal(err)
	}

	boom := errors.New("transient backend error")
	mem.fetchErr = boom
	bar := makePkg(t, dir, "bar", "1.0-1")
	err := repository.RepoAdd(
		"r", "x86_64", openSeek(t, bar), nil, false, nil,
	)
	mem.fetchErr = nil
	if !errors.Is(err, boom) {
		t.Fatalf("transient fetch error = %v, want boom", err)
	}

	remote, err := repository.RemoteRepo("r", "x86_64")
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, pkg := range remote.Pkgs {
		names[pkg.Name()] = true
	}
	if !names["foo"] || names["bar"] {
		t.Fatalf("transient error corrupted db: %v, want only foo", sortedKeys(names))
	}
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
