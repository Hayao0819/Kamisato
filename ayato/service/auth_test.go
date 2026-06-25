package service_test

import (
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

// newAuthService builds a real service backed by a badgerkv-backed
// AuthRepository so the allowlist use cases exercise the full
// service -> repository -> kv path.
func newAuthService(t *testing.T) service.Servicer {
	t.Helper()
	store, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatalf("badgerkv.New: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	authRepo := repository.NewAuthRepository(store)
	return service.New(nil, nil, authRepo, &conf.AyatoConfig{})
}

func TestServiceSeedBootstrapAdmin(t *testing.T) {
	s := newAuthService(t)
	if err := s.SeedBootstrapAdmin(777); err != nil {
		t.Fatalf("SeedBootstrapAdmin: %v", err)
	}
	if !s.IsAdmin(777) {
		t.Fatalf("bootstrap id 777 must be seeded and allowed")
	}
	admins, err := s.ListAdmins()
	if err != nil {
		t.Fatalf("ListAdmins: %v", err)
	}
	if len(admins) != 1 || admins[0].ID != 777 {
		t.Fatalf("ListAdmins = %+v, want exactly [777]", admins)
	}
}

func TestServiceSeedBootstrapNoopWhenNonEmpty(t *testing.T) {
	s := newAuthService(t)
	if err := s.AddAdmin(5, "bob"); err != nil {
		t.Fatalf("AddAdmin: %v", err)
	}
	// Seeding a different id must NOT add it when the allowlist is non-empty.
	if err := s.SeedBootstrapAdmin(999); err != nil {
		t.Fatalf("SeedBootstrapAdmin: %v", err)
	}
	if s.IsAdmin(999) {
		t.Fatalf("bootstrap must not seed into a non-empty allowlist")
	}
	// A non-positive bootstrap id is ignored.
	if err := s.SeedBootstrapAdmin(0); err != nil {
		t.Fatalf("SeedBootstrapAdmin(0): %v", err)
	}
}
