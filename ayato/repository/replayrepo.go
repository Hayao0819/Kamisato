package repository

import (
	"errors"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
)

// replayNS holds redeemed one-time code ids. An entry means the code was already
// exchanged; its TTL equals the code's remaining lifetime so it self-evicts once
// the code would have expired anyway (a stale id can no longer be replayed).
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
		return false, errwrap.NewErr("replay: empty id")
	}
	// A non-positive ttl means the code has already expired; verify rejects it
	// before we get here, so treat this as a no-op rather than recording the id
	// forever (a ttl of 0 is "no expiry" to the kv store).
	if ttl <= 0 {
		return true, nil
	}
	if adder, ok := r.kv.(kv.Adder); ok {
		return adder.Add(replayNS, id, []byte{1}, ttl)
	}
	// Fallback for a backend without an atomic insert: check then set. The residual
	// race is a single sub-second window between two exchanges of the same code; the
	// durable replay threat (redeeming a leaked code later) is still closed.
	if _, err := r.kv.Get(replayNS, id); err == nil {
		return false, nil
	} else if !errors.Is(err, kv.ErrNotFound) {
		return false, errwrap.WrapErr(err, "replay: get")
	}
	if err := r.kv.Set(replayNS, id, []byte{1}, ttl); err != nil {
		return false, errwrap.WrapErr(err, "replay: set")
	}
	return true, nil
}
