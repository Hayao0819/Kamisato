package service

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// Fail-closed: a non-positive id or a read miss returns false.
func (s *Service) IsAdmin(id int64) bool {
	return s.authRepo.IsAdmin(id)
}

// The caller is expected to have resolved any GitHub login to a numeric id first.
func (s *Service) AddAdmin(id int64, login string) error {
	s.adminMu.Lock()
	defer s.adminMu.Unlock()
	return s.authRepo.AddAdmin(id, login)
}

// RemoveAdmin refuses to empty the allowlist. Authentication fails closed on an
// empty list, and the bootstrap administrator is only seeded during startup.
func (s *Service) RemoveAdmin(id int64) error {
	s.adminMu.Lock()
	defer s.adminMu.Unlock()
	admins, err := s.authRepo.ListAdmins()
	if err != nil {
		return errors.WrapErr(err, "auth: list allowlist for removal")
	}
	if len(admins) == 1 && admins[0].ID == id {
		return fmt.Errorf("%w: cannot remove the last admin", domain.ErrConflict)
	}
	return s.authRepo.RemoveAdmin(id)
}

func (s *Service) ListAdmins() ([]domain.AllowedAdmin, error) {
	return s.authRepo.ListAdmins()
}

// SeedBootstrapAdmin seeds id only when the allowlist is empty; id <= 0 is
// ignored, leaving the allowlist empty (fail-closed: denies all).
func (s *Service) SeedBootstrapAdmin(id int64) error {
	if id <= 0 {
		return nil
	}
	s.adminMu.Lock()
	defer s.adminMu.Unlock()
	admins, err := s.authRepo.ListAdmins()
	if err != nil {
		return errors.WrapErr(err, "auth: list allowlist for seed")
	}
	if len(admins) == 0 {
		if err := s.authRepo.AddAdmin(id, ""); err != nil {
			return errors.WrapErr(err, "auth: seed bootstrap admin")
		}
	}
	return nil
}
