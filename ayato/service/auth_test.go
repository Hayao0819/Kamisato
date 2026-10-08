package service_test

import (
	"errors"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
	"github.com/Hayao0819/Kamisato/ayato/service"
)

// newAuthService builds a real badgerkv-backed service so tests exercise the
// full service -> repository -> kv path.
func newAuthService(t *testing.T) service.Servicer {
	t.Helper()
	store, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatalf("badgerkv.New: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	authRepo := repository.NewAuthRepository(store)
	return service.New(nil, nil, authRepo, nil, service.Settings{})
}

func TestRemoveAdminNeverEmptiesAllowlist(t *testing.T) {
	s := newAuthService(t)
	if err := s.AddAdmin(42, "alice"); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveAdmin(7); err != nil {
		t.Fatalf("remove absent admin: %v", err)
	}
	if err := s.RemoveAdmin(42); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("remove last admin = %v, want conflict", err)
	}
	if !s.IsAdmin(42) {
		t.Fatal("last administrator was removed")
	}
	if err := s.AddAdmin(7, "bob"); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveAdmin(42); err != nil {
		t.Fatalf("remove when another remains: %v", err)
	}
}

func TestConcurrentAdminRemovalsLeaveOneAdministrator(t *testing.T) {
	s := newAuthService(t)
	for _, id := range []int64{42, 7} {
		if err := s.AddAdmin(id, ""); err != nil {
			t.Fatal(err)
		}
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	for _, id := range []int64{42, 7} {
		go func() {
			<-start
			errs <- s.RemoveAdmin(id)
		}()
	}
	close(start)
	conflicts := 0
	for range 2 {
		if err := <-errs; errors.Is(err, domain.ErrConflict) {
			conflicts++
		} else if err != nil {
			t.Fatal(err)
		}
	}
	admins, err := s.ListAdmins()
	if err != nil {
		t.Fatal(err)
	}
	if conflicts != 1 || len(admins) != 1 {
		t.Fatalf("conflicts = %d, remaining administrators = %v", conflicts, admins)
	}
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
