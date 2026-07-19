package repository

import (
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// denyNS holds revoked token ids (jti); entries carry a TTL of the token's remaining
// lifetime so they self-evict once the token expires.
const (
	denyNS        = "deny"
	denySessionNS = "deny-session"
)

type DenylistRepository interface {
	// Revoke denylists jti for ttl. A non-positive ttl is a no-op: the token has
	// already expired and can no longer authenticate.
	Revoke(jti string, ttl time.Duration) error
	// Consume atomically records a one-time token.
	Consume(jti string, ttl time.Duration) (consumed bool, err error)
	IsRevoked(jti string) (bool, error)
	RevokeSession(sessionID string, ttl time.Duration) error
	IsSessionRevoked(sessionID string) (bool, error)
}

func (r *denylistRepository) Consume(jti string, ttl time.Duration) (bool, error) {
	if jti == "" {
		return false, errors.NewErr("deny: empty jti")
	}
	if ttl <= 0 {
		return false, nil
	}
	adder, ok := r.kv.(kv.Adder)
	if !ok {
		return false, errors.NewErr("deny: atomic token consumption is not supported by this store")
	}
	created, err := adder.Add(denyNS, jti, []byte{1}, ttl)
	if err != nil {
		return false, errors.WrapErr(err, "deny: consume jti")
	}
	return created, nil
}

type denylistRepository struct {
	kv kv.Store
}

func NewDenylistRepository(store kv.Store) DenylistRepository {
	return &denylistRepository{kv: store}
}

func (r *denylistRepository) Revoke(jti string, ttl time.Duration) error {
	return r.revoke(denyNS, "jti", jti, ttl)
}

func (r *denylistRepository) IsRevoked(jti string) (bool, error) {
	return r.isRevoked(denyNS, jti)
}

func (r *denylistRepository) RevokeSession(sessionID string, ttl time.Duration) error {
	return r.revoke(denySessionNS, "session id", sessionID, ttl)
}

func (r *denylistRepository) IsSessionRevoked(sessionID string) (bool, error) {
	return r.isRevoked(denySessionNS, sessionID)
}

func (r *denylistRepository) revoke(namespace, field, id string, ttl time.Duration) error {
	if id == "" {
		return errors.NewErr("deny: empty " + field)
	}
	if ttl <= 0 {
		return nil
	}
	return r.kv.Set(namespace, id, []byte{1}, ttl)
}

func (r *denylistRepository) isRevoked(namespace, id string) (bool, error) {
	if id == "" {
		return false, nil
	}
	_, err := r.kv.Get(namespace, id)
	if errors.Is(err, kv.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, errors.WrapErr(err, "deny: check jti")
	}
	return true, nil
}
