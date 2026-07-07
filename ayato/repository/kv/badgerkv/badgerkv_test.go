//go:build !js

package badgerkv_test

import (
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
)

// TestCloseStopsGCLoop verifies Close stops the background value-log GC goroutine
// cleanly: it returns promptly (not blocking on the ticker) and is safe to call
// more than once. Close waits on the goroutine, so a prompt return proves it exited.
func TestCloseStopsGCLoop(t *testing.T) {
	s, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for i, v := range []string{"a", "b", "c"} {
		if err := s.Set("ns", "k", []byte(v), 0); err != nil {
			t.Fatalf("Set %d: %v", i, err)
		}
	}
	if err := s.Delete("ns", "k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- s.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Close did not return promptly; GC goroutine likely leaked")
	}

	// Idempotent: a second Close must not panic (double-close of stop channel).
	if err := s.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

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
	// immediately. Use a 1s TTL and poll past it.
	if err := s.Set("ns", "tmp", []byte("v"), 1*time.Second); err != nil {
		t.Fatalf("Set with ttl: %v", err)
	}
	if _, err := s.Get("ns", "tmp"); err != nil {
		t.Fatalf("Get before expiry: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, err := s.Get("ns", "tmp")
		if errors.Is(err, kv.ErrNotFound) {
			break
		}
		if err != nil {
			t.Fatalf("Get during expiry wait: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatal("key did not expire within the deadline")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestAddIsFirstWriterWins covers the atomic set-if-absent primitive the one-time
// code guard relies on.
func TestAddIsFirstWriterWins(t *testing.T) {
	s, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	created, err := s.Add("ns", "code", []byte("first"), time.Minute)
	if err != nil {
		t.Fatalf("first Add: %v", err)
	}
	if !created {
		t.Fatal("first Add should report created")
	}

	created, err = s.Add("ns", "code", []byte("second"), time.Minute)
	if err != nil {
		t.Fatalf("second Add: %v", err)
	}
	if created {
		t.Fatal("second Add of an existing key must report not-created")
	}

	got, err := s.Get("ns", "code")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "first" {
		t.Fatalf("value = %q, want the first writer's value (no overwrite)", got)
	}
}
