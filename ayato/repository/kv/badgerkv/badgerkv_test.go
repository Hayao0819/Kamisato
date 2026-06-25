package badgerkv_test

import (
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
)

func newStore(t *testing.T) *badgerkv.Store {
	t.Helper()
	s, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
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

	if err := s.Delete("ns", "k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get("ns", "k"); !errors.Is(err, kv.ErrNotFound) {
		t.Fatalf("Get after Delete = %v, want ErrNotFound", err)
	}
	// Deleting a missing key is not an error.
	if err := s.Delete("ns", "k"); err != nil {
		t.Fatalf("Delete(missing): %v", err)
	}
}

func TestNamespaceIsolation(t *testing.T) {
	s := newStore(t)
	if err := s.Set("a", "k", []byte("1"), 0); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("b", "k", []byte("2"), 0); err != nil {
		t.Fatal(err)
	}
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
	if entries[0].Key != "a" || string(entries[0].Value) != "1" {
		t.Fatalf("entries[0] = %+v", entries[0])
	}
	if entries[1].Key != "b" || string(entries[1].Value) != "2" {
		t.Fatalf("entries[1] = %+v", entries[1])
	}
}

func TestTTLExpiry(t *testing.T) {
	s := newStore(t)
	// Badger's TTL has second resolution, so a sub-second TTL would expire
	// immediately. Use a 1s TTL and sleep past it.
	if err := s.Set("ns", "tmp", []byte("v"), 1*time.Second); err != nil {
		t.Fatalf("Set with ttl: %v", err)
	}
	if _, err := s.Get("ns", "tmp"); err != nil {
		t.Fatalf("Get before expiry: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)
	if _, err := s.Get("ns", "tmp"); !errors.Is(err, kv.ErrNotFound) {
		t.Fatalf("Get after expiry = %v, want ErrNotFound", err)
	}
}
