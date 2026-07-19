package repository

import (
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
)

// ReplayGuard records a one-time code id at redemption so a second exchange of the
// same code is rejected. It keeps ayato stateless: the "used" set lives in the
// shared kv, not in process memory.
type ReplayGuard interface {
	// Consume records id and reports whether this call was the first to do so. A
	// false firstUse means the id was already present — a replay.
	Consume(id string, ttl time.Duration) (firstUse bool, err error)
}

type replayGuard struct {
	kv kv.Store
}

func NewReplayGuard(store kv.Store) ReplayGuard {
	return &replayGuard{kv: store}
}

func (r *replayGuard) Consume(id string, ttl time.Duration) (bool, error) {
	return replayConsumption.consume(r.kv, id, ttl)
}

type consumptionPolicy struct {
	namespace    string
	emptyError   string
	errorContext string
}

var replayConsumption = consumptionPolicy{
	namespace:    kv.ReplayCodes,
	emptyError:   "replay: empty id",
	errorContext: "replay: consume",
}

// consume is the shared first-writer-wins operation for one-time credentials. A
// backend without atomic Add support fails closed.
func (p consumptionPolicy) consume(
	store kv.Store,
	key string,
	ttl time.Duration,
) (bool, error) {
	if key == "" {
		return false, errors.NewErr(p.emptyError)
	}
	if ttl <= 0 {
		return false, nil
	}
	adder, ok := store.(kv.Adder)
	if !ok {
		return false, errors.NewErr("atomic consumption is not supported by this store")
	}
	consumed, err := adder.Add(p.namespace, key, []byte{1}, ttl)
	if err != nil {
		return false, errors.WrapErr(err, p.errorContext)
	}
	return consumed, nil
}
