package repository

import (
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/schema"
	"github.com/Hayao0819/Kamisato/internal/errors"
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
	return tokenConsumption.consume(r.kv, jti, ttl)
}

var tokenConsumption = consumptionPolicy{
	namespace:    schema.TokenDenylist,
	emptyError:   "deny: empty jti",
	errorContext: "deny: consume jti",
}

type denylistRepository struct {
	kv kv.Store
}

func NewDenylistRepository(store kv.Store) DenylistRepository {
	return &denylistRepository{kv: store}
}

func (r *denylistRepository) Revoke(jti string, ttl time.Duration) error {
	return r.revoke(schema.TokenDenylist, "jti", jti, ttl)
}

func (r *denylistRepository) IsRevoked(jti string) (bool, error) {
	return r.isRevoked(schema.TokenDenylist, jti)
}

func (r *denylistRepository) RevokeSession(sessionID string, ttl time.Duration) error {
	return r.revoke(schema.SessionDenylist, "session id", sessionID, ttl)
}

func (r *denylistRepository) IsSessionRevoked(sessionID string) (bool, error) {
	return r.isRevoked(schema.SessionDenylist, sessionID)
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
