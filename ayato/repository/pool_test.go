package repository

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob/localfs"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

const poolTestRepo = "testrepo"

func newPoolStore_test(t *testing.T) (*poolStore, kv.Store) {
	t.Helper()
	kvStore, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatalf("open badger: %v", err)
	}
	t.Cleanup(func() { _ = kvStore.Close() })
	under := localfs.New(t.TempDir(), []string{poolTestRepo, poolRepo})
	return newPoolStore(under, kvStore), kvStore
}

func pkgFile(name string, content []byte) stream.SeekFile {
	return stream.NewFileStream(name, "application/zstd", nopSeekCloser{bytes.NewReader(content)})
}

func fetchBytes(t *testing.T, p *poolStore, repo, arch, name string) []byte {
	t.Helper()
	f, err := p.FetchFile(repo, arch, name)
	if err != nil {
		t.Fatalf("fetch %s/%s/%s: %v", repo, arch, name, err)
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return b
}

// TestPoolDedupAndServe stores two identical-content packages under different
// names, then checks there is ONE pool object, TWO pointers, and that a fetch of
// either name resolves through the pool to the shared bytes.
func TestPoolDedupAndServe(t *testing.T) {
	p, kvStore := newPoolStore_test(t)
	content := bytes.Repeat([]byte("kamisato"), 512)

	a := "libfoo-1-1-x86_64.pkg.tar.zst"
	b := "libfoo-copy-1-1-x86_64.pkg.tar.zst"
	if err := p.StoreFile(poolTestRepo, "x86_64", pkgFile(a, content)); err != nil {
		t.Fatalf("store a: %v", err)
	}
	if err := p.StoreFile(poolTestRepo, "x86_64", pkgFile(b, content)); err != nil {
		t.Fatalf("store b: %v", err)
	}

	objs, err := kvStore.List(poolObjNS)
	if err != nil {
		t.Fatalf("list objects: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("pool objects = %d, want 1 (identical content deduped)", len(objs))
	}
	ptrs, err := kvStore.List(poolPtrNS)
	if err != nil {
		t.Fatalf("list pointers: %v", err)
	}
	if len(ptrs) != 2 {
		t.Fatalf("pointers = %d, want 2", len(ptrs))
	}

	if got := fetchBytes(t, p, poolTestRepo, "x86_64", a); !bytes.Equal(got, content) {
		t.Fatal("fetch a: bytes differ from stored content")
	}
	if got := fetchBytes(t, p, poolTestRepo, "x86_64", b); !bytes.Equal(got, content) {
		t.Fatal("fetch b: bytes differ from stored content")
	}

	// The pooled names show up in the listing even though their bytes live in the
	// pool, and the reserved pool repo is hidden from the servable repo set.
	files, err := p.Files(poolTestRepo, "x86_64")
	if err != nil {
		t.Fatalf("files: %v", err)
	}
	if !contains(files, a) || !contains(files, b) {
		t.Fatalf("Files = %v, want both %s and %s", files, a, b)
	}
	names, err := p.RepoNames()
	if err != nil {
		t.Fatalf("repo names: %v", err)
	}
	if contains(names, poolRepo) {
		t.Fatalf("RepoNames = %v, must not expose the reserved pool repo", names)
	}
}

// TestPoolFallbackToLegacy proves a directly-stored (pre-pool) file with no
// pointer still serves, so already-published repos keep working.
func TestPoolFallbackToLegacy(t *testing.T) {
	p, _ := newPoolStore_test(t)
	// A db artifact is never pooled: it passes through and serves from its path.
	dbName := poolTestRepo + ".db.tar.gz"
	if err := p.StoreFile(poolTestRepo, "x86_64", pkgFile(dbName, []byte("dbdata"))); err != nil {
		t.Fatalf("store db: %v", err)
	}
	if got := fetchBytes(t, p, poolTestRepo, "x86_64", dbName); !bytes.Equal(got, []byte("dbdata")) {
		t.Fatal("db artifact did not serve through the passthrough path")
	}
}

// TestPoolGC checks the retention GC: an unreferenced object is deleted, a
// referenced one is kept, and keep-N retains the newest unreferenced versions.
func TestPoolGC(t *testing.T) {
	p, kvStore := newPoolStore_test(t)
	clock := time.Unix(1_700_000_000, 0)
	p.now = func() time.Time { return clock }

	// Three versions of one pkgbase, each distinct content (distinct hash), stored
	// at increasing times so keep-N can order them.
	store := func(name string, content []byte) {
		if err := p.StoreFile(poolTestRepo, "x86_64", pkgFile(name, content)); err != nil {
			t.Fatalf("store %s: %v", name, err)
		}
		clock = clock.Add(time.Minute)
	}
	v1 := "pkg-1-1-x86_64.pkg.tar.zst"
	v2 := "pkg-2-1-x86_64.pkg.tar.zst"
	v3 := "pkg-3-1-x86_64.pkg.tar.zst"
	store(v1, []byte("one"))
	store(v2, []byte("two"))
	store(v3, []byte("three"))

	hashV1 := mustPointer(t, kvStore, poolTestRepo, "x86_64", v1)

	// Drop v1 and v2 (unreferenced); v3 stays referenced.
	if err := p.DeleteFile(poolTestRepo, "x86_64", v1); err != nil {
		t.Fatalf("delete v1 pointer: %v", err)
	}
	if err := p.DeleteFile(poolTestRepo, "x86_64", v2); err != nil {
		t.Fatalf("delete v2 pointer: %v", err)
	}

	// keep-N=1: among unreferenced {v1,v2}, keep the newest (v2), delete v1. v3 is
	// referenced and always kept. Window 0 makes them eligible in one pass.
	res, err := p.CollectPool(context.Background(), PoolPolicy{KeepUnreferenced: 1, RetentionWindow: 0})
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if res.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1 (only the oldest unreferenced version)", res.Deleted)
	}

	// The deleted object's bytes are gone; v2 and v3 remain.
	if _, err := p.Store.FetchFile(poolRepo, poolArch, hashV1); !errors.Is(err, blob.ErrNotFound) {
		t.Fatalf("v1 pool bytes: err = %v, want ErrNotFound (collected)", err)
	}
	if got := fetchBytes(t, p, poolTestRepo, "x86_64", v3); !bytes.Equal(got, []byte("three")) {
		t.Fatal("referenced v3 was collected or corrupted")
	}
	objs, err := kvStore.List(poolObjNS)
	if err != nil {
		t.Fatalf("list objects: %v", err)
	}
	if len(objs) != 2 {
		t.Fatalf("remaining pool objects = %d, want 2 (v2 retained, v3 referenced)", len(objs))
	}
}

// TestPoolGCGraceWindow proves the retention window keeps a just-unreferenced
// object until the grace elapses.
func TestPoolGCGraceWindow(t *testing.T) {
	p, _ := newPoolStore_test(t)
	clock := time.Unix(1_700_000_000, 0)
	p.now = func() time.Time { return clock }

	name := "solo-1-1-x86_64.pkg.tar.zst"
	if err := p.StoreFile(poolTestRepo, "x86_64", pkgFile(name, []byte("solo"))); err != nil {
		t.Fatalf("store: %v", err)
	}
	if err := p.DeleteFile(poolTestRepo, "x86_64", name); err != nil {
		t.Fatalf("delete pointer: %v", err)
	}

	window := time.Hour
	// Within the grace window: nothing collected (the clock starts on first observe).
	res, err := p.CollectPool(context.Background(), PoolPolicy{RetentionWindow: window})
	if err != nil {
		t.Fatalf("collect (grace): %v", err)
	}
	if res.Deleted != 0 {
		t.Fatalf("deleted = %d within grace, want 0", res.Deleted)
	}

	// Past the window: it is collected.
	clock = clock.Add(window + time.Minute)
	res, err = p.CollectPool(context.Background(), PoolPolicy{RetentionWindow: window})
	if err != nil {
		t.Fatalf("collect (post-grace): %v", err)
	}
	if res.Deleted != 1 {
		t.Fatalf("deleted = %d past grace, want 1", res.Deleted)
	}
}

func mustPointer(t *testing.T, s kv.Store, repo, arch, name string) string {
	t.Helper()
	v, err := s.Get(poolPtrNS, ptrKey(repo, arch, name))
	if err != nil {
		t.Fatalf("pointer %s: %v", name, err)
	}
	return string(v)
}
