package repository

import (
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
)

// kvSpy is a minimal in-memory kv.Store that counts single-key writes.
type kvSpy struct {
	data map[string]string
	sets int
}

func newKVSpy() *kvSpy { return &kvSpy{data: map[string]string{}} }

func (s *kvSpy) Get(ns, key string) ([]byte, error) {
	v, ok := s.data[ns+"\x00"+key]
	if !ok {
		return nil, kv.ErrNotFound
	}
	return []byte(v), nil
}

func (s *kvSpy) Set(ns, key string, value []byte, _ time.Duration) error {
	s.sets++
	s.data[ns+"\x00"+key] = string(value)
	return nil
}

func (s *kvSpy) Delete(ns, key string) error     { delete(s.data, ns+"\x00"+key); return nil }
func (s *kvSpy) List(string) ([]kv.Entry, error) { return nil, nil }
func (s *kvSpy) Close() error                    { return nil }

// bulkKVSpy also satisfies kv.BulkStore, so StorePackageFiles must take the bulk path.
type bulkKVSpy struct {
	*kvSpy
	bulkCalls  int
	bulkWrites int
}

func newBulkKVSpy() *bulkKVSpy { return &bulkKVSpy{kvSpy: newKVSpy()} }

func (s *bulkKVSpy) BulkSet(ns string, entries []kv.Entry, _ time.Duration) error {
	s.bulkCalls++
	s.bulkWrites += len(entries)
	for _, e := range entries {
		s.data[ns+"\x00"+e.Key] = string(e.Value)
	}
	return nil
}

func (s *bulkKVSpy) BulkDelete(ns string, keys []string) error {
	for _, k := range keys {
		delete(s.data, ns+"\x00"+k)
	}
	return nil
}

var twoEntries = []PackageFileEntry{
	{Arch: "x86_64", Name: "foo", FileName: "foo-1.0-1-x86_64.pkg.tar.zst"},
	{Arch: "any", Name: "bar", FileName: "bar-1.0-1-any.pkg.tar.zst"},
}

// A bulk-capable backend collapses the whole batch into one request, and the
// entries stay readable through PackageFile afterwards.
func TestStorePackageFiles_UsesBulkOnce(t *testing.T) {
	spy := newBulkKVSpy()
	r := NewPackageMetadataRepo(spy)

	if err := r.StorePackageFiles("myrepo", twoEntries); err != nil {
		t.Fatalf("StorePackageFiles: %v", err)
	}
	if spy.bulkCalls != 1 {
		t.Errorf("BulkSet called %d times, want 1", spy.bulkCalls)
	}
	if spy.bulkWrites != 2 {
		t.Errorf("BulkSet wrote %d entries, want 2", spy.bulkWrites)
	}
	if spy.sets != 0 {
		t.Errorf("fell back to %d single Set calls, want 0", spy.sets)
	}
	if got, _ := r.PackageFile("myrepo", "x86_64", "foo"); got != "foo-1.0-1-x86_64.pkg.tar.zst" {
		t.Errorf("PackageFile(foo) = %q after bulk write", got)
	}
}

// A backend without BulkStore still stores every entry, one Set per key.
func TestStorePackageFiles_FallsBackPerKey(t *testing.T) {
	spy := newKVSpy()
	r := NewPackageMetadataRepo(spy)

	if err := r.StorePackageFiles("myrepo", twoEntries); err != nil {
		t.Fatalf("StorePackageFiles: %v", err)
	}
	if spy.sets != 2 {
		t.Errorf("single Set calls = %d, want 2", spy.sets)
	}
	if got, _ := r.PackageFile("myrepo", "any", "bar"); got != "bar-1.0-1-any.pkg.tar.zst" {
		t.Errorf("PackageFile(bar) = %q after per-key write", got)
	}
}

func TestStorePackageFiles_EmptyIsNoop(t *testing.T) {
	spy := newBulkKVSpy()
	r := NewPackageMetadataRepo(spy)

	if err := r.StorePackageFiles("myrepo", nil); err != nil {
		t.Fatalf("StorePackageFiles(nil): %v", err)
	}
	if spy.bulkCalls != 0 || spy.sets != 0 {
		t.Errorf("empty batch wrote something: bulk=%d set=%d", spy.bulkCalls, spy.sets)
	}
}
