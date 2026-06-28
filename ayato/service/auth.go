package service

import (
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/internal/utils"
)

// Fail-closed: a non-positive id or a read miss returns false.
func (s *Service) IsAdmin(id int64) bool {
	return s.authRepo.IsAdmin(id)
}

// The caller is expected to have resolved any GitHub login to a numeric id first.
func (s *Service) AddAdmin(id int64, login string) error {
	return s.authRepo.AddAdmin(id, login)
}

func (s *Service) RemoveAdmin(id int64) error {
	return s.authRepo.RemoveAdmin(id)
}

func (s *Service) ListAdmins() ([]repository.AllowedAdmin, error) {
	return s.authRepo.ListAdmins()
}

// SeedBootstrapAdmin seeds id onto the allowlist when id > 0 and the allowlist is
// currently empty, matching the previous on-disk bootstrap behaviour. A
// bootstrap id <= 0 is ignored (no seed), leaving the allowlist empty
// (fail-closed: denies all).
func (s *Service) SeedBootstrapAdmin(id int64) error {
	if id <= 0 {
		return nil
	}
	admins, err := s.authRepo.ListAdmins()
	if err != nil {
		return utils.WrapErr(err, "auth: list allowlist for seed")
	}
	if len(admins) == 0 {
		if err := s.authRepo.AddAdmin(id, ""); err != nil {
			return utils.WrapErr(err, "auth: seed bootstrap admin")
		}
	}
	return nil
}
