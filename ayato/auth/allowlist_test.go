package auth

import (
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/kv/badgerkv"
)

func newTestAllowlist(t *testing.T, bootstrap int64) *AllowlistRepo {
	t.Helper()
	store, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatalf("badgerkv.New: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repo := NewAllowlistRepo(store)
	if err := SeedBootstrap(repo, bootstrap); err != nil {
		t.Fatalf("SeedBootstrap: %v", err)
	}
	return repo
}

func TestAllowlistFailClosed(t *testing.T) {
	// Empty allowlist (no bootstrap) denies everyone.
	r := newTestAllowlist(t, 0)
	if r.Has(1) {
		t.Fatalf("empty allowlist must deny id 1")
	}
	if r.Has(0) || r.Has(-5) {
		t.Fatalf("non-positive ids must be denied")
	}

	// Add rejects non-positive ids.
	if err := r.Add(0, "x"); err == nil {
		t.Fatalf("Add(0) must be rejected")
	}
	if err := r.Add(-1, "x"); err == nil {
		t.Fatalf("Add(-1) must be rejected")
	}

	// Unknown id is denied even with another id present.
	if err := r.Add(42, "alice"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if r.Has(99) {
		t.Fatalf("unknown id 99 must be denied")
	}
	if !r.Has(42) {
		t.Fatalf("added id 42 must be allowed")
	}
}

func TestAllowlistBootstrapSeed(t *testing.T) {
	r := newTestAllowlist(t, 777)
	if !r.Has(777) {
		t.Fatalf("bootstrap id 777 must be seeded and allowed")
	}
	admins, err := r.ListAllowed()
	if err != nil {
		t.Fatalf("ListAllowed: %v", err)
	}
	if len(admins) != 1 || admins[0].ID != 777 {
		t.Fatalf("ListAllowed = %+v, want exactly [777]", admins)
	}
}

func TestSeedBootstrapNoopWhenNonEmpty(t *testing.T) {
	r := newTestAllowlist(t, 0)
	if err := r.Add(5, "bob"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	// Seeding a different id must NOT add it when the allowlist is already non-empty.
	if err := SeedBootstrap(r, 999); err != nil {
		t.Fatalf("SeedBootstrap: %v", err)
	}
	if r.Has(999) {
		t.Fatalf("bootstrap must not seed into a non-empty allowlist")
	}
	// A non-positive bootstrap id is ignored.
	if err := SeedBootstrap(r, 0); err != nil {
		t.Fatalf("SeedBootstrap(0): %v", err)
	}
}

func TestAllowlistRemove(t *testing.T) {
	r := newTestAllowlist(t, 0)
	_ = r.Add(5, "bob")
	if !r.Has(5) {
		t.Fatalf("id 5 must be allowed after Add")
	}
	if err := r.Remove(5); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if r.Has(5) {
		t.Fatalf("id 5 must be denied after Remove")
	}
}

func TestAllowlistList(t *testing.T) {
	r := newTestAllowlist(t, 0)
	_ = r.Add(1, "one")
	_ = r.Add(2, "two")
	admins, err := r.ListAllowed()
	if err != nil {
		t.Fatalf("ListAllowed: %v", err)
	}
	got := map[int64]string{}
	for _, a := range admins {
		got[a.ID] = a.Login
	}
	if got[1] != "one" || got[2] != "two" || len(got) != 2 {
		t.Fatalf("ListAllowed = %+v, want {1:one, 2:two}", got)
	}
}
