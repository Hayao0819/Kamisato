package service

import (
	"fmt"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository"
)

// WithDenylist attaches the per-token revocation store; nil (unset) means no
// per-token revocation is wired.
func (s *Service) WithDenylist(dl repository.DenylistRepository) *Service {
	s.denylistRepo = dl
	return s
}

// IsRevoked reports whether the token id (jti) was individually revoked; it is
// false when no denylist is wired.
func (s *Service) IsRevoked(jti string) bool {
	return s.denylistRepo != nil && s.denylistRepo.IsRevoked(jti)
}

// Revoke denylists jti for ttl. It errors when no denylist is wired.
func (s *Service) Revoke(jti string, ttl time.Duration) error {
	if s.denylistRepo == nil {
		return fmt.Errorf("token revocation is not configured")
	}
	return s.denylistRepo.Revoke(jti, ttl)
}
