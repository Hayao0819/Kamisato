package migrate_test

import (
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/migrate"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
)

func TestLayoutVersion(t *testing.T) {
	s := newStore(t)

	if v, err := migrate.ReadLayout(s); err != nil || v != 0 {
		t.Fatalf("ReadLayout unset = (%d, %v), want (0, nil)", v, err)
	}
	if err := migrate.WriteLayout(s, 3); err != nil {
		t.Fatalf("WriteLayout: %v", err)
	}
	if v, err := migrate.ReadLayout(s); err != nil || v != 3 {
		t.Fatalf("ReadLayout = (%d, %v), want (3, nil)", v, err)
	}

	if v, in, _ := migrate.Guard(s, 2, 4); v != 3 || !in {
		t.Fatalf("Guard(2,4) = (%d, %v), want (3, true)", v, in)
	}
	if _, in, _ := migrate.Guard(s, 5, 6); in {
		t.Fatal("Guard(5,6) inRange = true, want false")
	}
}

// TestBulkFallback covers the per-key path a backend without kv.BulkStore takes.
func TestBulkFallback(t *testing.T) {
	s := newStore(t)
	entries := []kv.Entry{{Key: "a", Value: []byte("1")}, {Key: "b", Value: []byte("2")}}

	if err := migrate.BulkSet(s, "ns", entries, 0); err != nil {
		t.Fatalf("BulkSet: %v", err)
	}
	got, err := s.List("ns")
	if err != nil || len(got) != 2 {
		t.Fatalf("List after BulkSet = (%v, %v), want 2 entries", got, err)
	}

	if err := migrate.BulkDelete(s, "ns", []string{"a", "b"}); err != nil {
		t.Fatalf("BulkDelete: %v", err)
	}
	if got, _ := s.List("ns"); len(got) != 0 {
		t.Fatalf("List after BulkDelete = %v, want empty", got)
	}
}

func newStore(t *testing.T) kv.Store {
	t.Helper()
	s, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatalf("badgerkv.New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
