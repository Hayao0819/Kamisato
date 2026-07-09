//go:build !js

package sqlkv_test

import (
	"sort"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/sqlkv"
)

func newStore(t *testing.T) *sqlkv.Store {
	t.Helper()
	// In-memory database per test; skip with a clear note if the gorm sqlite driver is absent.
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite driver unavailable, skipping sqlkv tests: %v", err)
	}
	s, err := sqlkv.NewWithDB(db)
	if err != nil {
		t.Fatalf("NewWithDB: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestGetSetDelete(t *testing.T) {
	s := newStore(t)

	if _, err := s.Get("ns", "missing"); !errors.Is(err, kv.ErrNotFound) {
		t.Fatalf("Get(missing) = %v, want ErrNotFound", err)
	}

	if err := s.Set("ns", "k", []byte("v"), 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.Get("ns", "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "v" {
		t.Fatalf("Get = %q, want %q", got, "v")
	}

	// Upsert overwrites.
	if err := s.Set("ns", "k", []byte("v2"), 0); err != nil {
		t.Fatalf("Set upsert: %v", err)
	}
	if got, _ := s.Get("ns", "k"); string(got) != "v2" {
		t.Fatalf("Get after upsert = %q, want v2", got)
	}

	if err := s.Delete("ns", "k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get("ns", "k"); !errors.Is(err, kv.ErrNotFound) {
		t.Fatalf("Get after Delete = %v, want ErrNotFound", err)
	}
}

func TestNamespaceIsolation(t *testing.T) {
	s := newStore(t)
	_ = s.Set("a", "k", []byte("1"), 0)
	_ = s.Set("b", "k", []byte("2"), 0)
	if v, _ := s.Get("a", "k"); string(v) != "1" {
		t.Fatalf("ns a = %q, want 1", v)
	}
	if v, _ := s.Get("b", "k"); string(v) != "2" {
		t.Fatalf("ns b = %q, want 2", v)
	}
}

func TestList(t *testing.T) {
	s := newStore(t)
	_ = s.Set("ns", "a", []byte("1"), 0)
	_ = s.Set("ns", "b", []byte("2"), 0)
	_ = s.Set("other", "c", []byte("3"), 0)

	entries, err := s.List("ns")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	if len(entries) != 2 {
		t.Fatalf("List len = %d, want 2 (got %+v)", len(entries), entries)
	}
	if entries[0].Key != "a" || entries[1].Key != "b" {
		t.Fatalf("List keys = %+v", entries)
	}
}

func TestTTLExpiry(t *testing.T) {
	s := newStore(t)
	if err := s.Set("ns", "tmp", []byte("v"), 50*time.Millisecond); err != nil {
		t.Fatalf("Set with ttl: %v", err)
	}
	if _, err := s.Get("ns", "tmp"); err != nil {
		t.Fatalf("Get before expiry: %v", err)
	}
	// Poll past the 50ms TTL until the row expires instead of sleeping a fixed span.
	deadline := time.Now().Add(2 * time.Second)
	for {
		_, err := s.Get("ns", "tmp")
		if errors.Is(err, kv.ErrNotFound) {
			break
		}
		if err != nil {
			t.Fatalf("Get during expiry wait: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatal("row did not expire within the deadline")
		}
		time.Sleep(5 * time.Millisecond)
	}
	// An expired row is excluded from List too.
	if entries, _ := s.List("ns"); len(entries) != 0 {
		t.Fatalf("List after expiry = %+v, want empty", entries)
	}
}
