package repository

import (
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/schema"
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
	if id == "" {
		return false, errors.NewErr("replay: empty id")
	}
	consumed, err := consumeOnce(r.kv, schema.ReplayCodes, id, ttl)
	if err != nil {
		return false, errors.WrapErr(err, "replay: consume")
	}
	return consumed, nil
}

// consumeOnce is the shared first-writer-wins primitive for one-time credentials.
// A backend without atomic Add support must fail closed.
func consumeOnce(
	store kv.Store,
	namespace, key string,
	ttl time.Duration,
) (bool, error) {
	if ttl <= 0 {
		return false, nil
	}
	adder, ok := store.(kv.Adder)
	if !ok {
		return false, errors.NewErr("atomic consumption is not supported by this store")
	}
	return adder.Add(namespace, key, []byte{1}, ttl)
}
