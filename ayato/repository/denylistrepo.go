package repository

import (
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
)

// denyNS holds revoked token ids (jti); entries carry a TTL of the token's remaining
// lifetime so they self-evict once the token expires.
const denyNS = "deny"

type DenylistRepository interface {
	// Revoke denylists jti for ttl. A non-positive ttl is a no-op: the token has
	// already expired and can no longer authenticate.
	Revoke(jti string, ttl time.Duration) error
	IsRevoked(jti string) bool
}

type denylistRepository struct {
	kv kv.Store
}

func NewDenylistRepository(store kv.Store) DenylistRepository {
	return &denylistRepository{kv: store}
}

func (r *denylistRepository) Revoke(jti string, ttl time.Duration) error {
	if jti == "" {
		return errwrap.NewErr("deny: empty jti")
	}
	// A ttl of 0 means "no expiry" to the kv store, so skip an already-expired
	// token rather than denylist it forever.
	if ttl <= 0 {
		return nil
	}
	return r.kv.Set(denyNS, jti, []byte{1}, ttl)
}

func (r *denylistRepository) IsRevoked(jti string) bool {
	if jti == "" {
		return false
	}
	_, err := r.kv.Get(denyNS, jti)
	return err == nil
}
