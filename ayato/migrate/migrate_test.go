package migrate_test

import (
	"context"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/migrate"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
)

type fakeMig struct {
	ver                int
	expands, contracts *int
}

func (f fakeMig) Version() int                                        { return f.ver }
func (f fakeMig) Name() string                                        { return "fake" }
func (f fakeMig) Expand(_ context.Context, _ *migrate.Stores) error   { *f.expands++; return nil }
func (f fakeMig) Contract(_ context.Context, _ *migrate.Stores) error { *f.contracts++; return nil }

func TestRunPhasesIdempotent(t *testing.T) {
	s := newStore(t)
	stores := &migrate.Stores{KV: s}
	var expands, contracts int
	ms := []migrate.Migration{fakeMig{ver: 1, expands: &expands, contracts: &contracts}}
	ctx := context.Background()

	// Expand runs once; the layout version does not advance yet.
	if _, err := migrate.Run(ctx, stores, ms, migrate.RunOptions{Phase: migrate.PhaseExpand}); err != nil {
		t.Fatalf("expand: %v", err)
	}
	if _, err := migrate.Run(ctx, stores, ms, migrate.RunOptions{Phase: migrate.PhaseExpand}); err != nil {
		t.Fatalf("expand rerun: %v", err)
	}
	if expands != 1 {
		t.Fatalf("expands = %d, want 1 (idempotent)", expands)
	}
	if v, _ := migrate.ReadLayout(s); v != 0 {
		t.Fatalf("layout after expand = %d, want 0", v)
	}

	// Contract runs once and advances the layout version.
	if _, err := migrate.Run(ctx, stores, ms, migrate.RunOptions{Phase: migrate.PhaseContract}); err != nil {
		t.Fatalf("contract: %v", err)
	}
	if _, err := migrate.Run(ctx, stores, ms, migrate.RunOptions{Phase: migrate.PhaseContract}); err != nil {
		t.Fatalf("contract rerun: %v", err)
	}
	if contracts != 1 {
		t.Fatalf("contracts = %d, want 1 (idempotent)", contracts)
	}
	if v, _ := migrate.ReadLayout(s); v != 1 {
		t.Fatalf("layout after contract = %d, want 1", v)
	}
}

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

// TestBulkDeleteFallback covers the per-key path a backend without kv.BulkStore takes.
func TestBulkDeleteFallback(t *testing.T) {
	s := newStore(t)
	for _, key := range []string{"a", "b"} {
		if err := s.Set("ns", key, []byte(key), 0); err != nil {
			t.Fatal(err)
		}
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
