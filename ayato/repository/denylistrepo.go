package repository

import (
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

type DenylistRepository = service.DenylistRepository

func (r *denylistRepository) Consume(jti string, ttl time.Duration) (bool, error) {
	return tokenConsumption.consume(r.kv, jti, ttl)
}

type revocationPolicy struct {
	namespace string
	field     string
}

var (
	tokenRevocations = revocationPolicy{
		namespace: kv.TokenDenylist,
		field:     "jti",
	}
	sessionRevocations = revocationPolicy{
		namespace: kv.SessionDenylist,
		field:     "session id",
	}
	tokenConsumption = consumptionPolicy{
		namespace:    tokenRevocations.namespace,
		emptyError:   "deny: empty jti",
		errorContext: "deny: consume jti",
	}
)

type denylistRepository struct {
	kv kv.Store
}

func NewDenylistRepository(store kv.Store) DenylistRepository {
	return &denylistRepository{kv: store}
}

func (r *denylistRepository) Revoke(jti string, ttl time.Duration) error {
	return tokenRevocations.revoke(r.kv, jti, ttl)
}

func (r *denylistRepository) IsRevoked(jti string) (bool, error) {
	return tokenRevocations.isRevoked(r.kv, jti)
}

func (r *denylistRepository) RevokeSession(sessionID string, ttl time.Duration) error {
	return sessionRevocations.revoke(r.kv, sessionID, ttl)
}

func (r *denylistRepository) IsSessionRevoked(sessionID string) (bool, error) {
	return sessionRevocations.isRevoked(r.kv, sessionID)
}

func (p revocationPolicy) revoke(store kv.Store, id string, ttl time.Duration) error {
	if id == "" {
		return errors.NewErr("deny: empty " + p.field)
	}
	if ttl <= 0 {
		return nil
	}
	return store.Set(p.namespace, id, []byte{1}, ttl)
}

func (p revocationPolicy) isRevoked(store kv.Store, id string) (bool, error) {
	if id == "" {
		return false, nil
	}
	_, revoked, err := getOptional(store, p.namespace, id, "deny: check id")
	if err != nil {
		return false, err
	}
	return revoked, nil
}
