package service

import (
	"github.com/Hayao0819/Kamisato/ayato/domain"
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

func (s *Service) ListAdmins() ([]domain.AllowedAdmin, error) {
	entries, err := s.authRepo.ListAdmins()
	if err != nil {
		return nil, err
	}
	admins := make([]domain.AllowedAdmin, 0, len(entries))
	for _, e := range entries {
		admins = append(admins, domain.AllowedAdmin{ID: e.ID, Login: e.Login})
	}
	return admins, nil
}

// SeedBootstrapAdmin seeds id only when the allowlist is empty; id <= 0 is
// ignored, leaving the allowlist empty (fail-closed: denies all).
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
