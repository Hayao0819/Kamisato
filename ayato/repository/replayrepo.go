package repository

import (
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
)

// replayNS holds redeemed one-time code ids; an entry means the code was already
// exchanged. Its TTL equals the code's remaining lifetime so it self-evicts once the
// code can no longer be replayed.
const replayNS = "replay"

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
	if ttl <= 0 {
		return false, nil
	}
	adder, ok := r.kv.(kv.Adder)
	if !ok {
		return false, errors.NewErr("replay: atomic consumption is not supported by this store")
	}
	return adder.Add(replayNS, id, []byte{1}, ttl)
}
