package platform

import (
	"strconv"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"
	sharedlimit "github.com/Hayao0819/Kamisato/pkg/ratelimit"
)

// ns partitions the limiter's counters from every other kv consumer.
const ns = "ratelimit"

// minTTL floors a counter's TTL: BadgerDB stores expiry at one-second granularity,
// so a sub-second window's counter would otherwise read back as already expired.
// The window boundary is carried by the KEY, not the TTL, so an over-long TTL only
// leaves a harmless stale counter the next window never consults.
const minTTL = 3 * time.Second

// RateLimitStore is the narrow persistence port required by RateLimiter.
type RateLimitStore interface {
	Get(namespace, key string) ([]byte, error)
	Set(namespace, key string, value []byte, ttl time.Duration) error
}

type rateLimitAdder interface {
	Add(namespace, key string, value []byte, ttl time.Duration) (bool, error)
}

// RateLimiter enforces a fixed-window request limit on a shared kv.Store. It is
// safe for concurrent use provided the store is.
type RateLimiter struct {
	store    RateLimitStore
	notFound error
	now      func() time.Time
}

func NewRateLimiter(store RateLimitStore, notFound error) *RateLimiter {
	return &RateLimiter{store: store, notFound: notFound, now: time.Now}
}

// Allow records one request for (scope, client) in the current window. Scope
// separates independent consumers sharing the store; client is the caller
// identity. On a kv error it fails open so a limiter outage does not turn every
// request into a server outage.
func (l *RateLimiter) Allow(scope, client string, policy sharedlimit.Policy) sharedlimit.Decision {
	if !policy.Enabled() {
		return sharedlimit.Decision{Allowed: true}
	}
	now := l.now()
	idx, retry := sharedlimit.WindowAt(now, policy.Window)
	key := scope + ":" + client + ":" + strconv.FormatInt(idx, 10)

	count, err := l.bump(key, policy.Limit, ttlFor(policy.Window))
	if err != nil {
		return sharedlimit.Decision{Allowed: true}
	}
	if count > policy.Limit {
		return sharedlimit.Decision{RetryAfter: retry}
	}
	return sharedlimit.Decision{Allowed: true}
}

// bump increments the window counter and returns the resulting count. The first
// request in a window is created atomically via kv.Adder (when offered), so two
// racing first requests cannot both under-count. Subsequent increments are a
// Get-then-Set read-modify-write: kv has no atomic add, so two concurrent
// increments can both read n and write n+1, admitting ONE extra request — a
// residual bounded by in-flight concurrency per key per window that never resets
// the limit. An eventually-consistent store (cfkv) widens it because a stale read
// can miss a just-written increment.
func (l *RateLimiter) bump(key string, limit int, ttl time.Duration) (int, error) {
	b, err := l.store.Get(ns, key)
	if errors.Is(err, l.notFound) {
		if adder, ok := l.store.(rateLimitAdder); ok {
			created, aerr := adder.Add(ns, key, itob(1), ttl)
			if aerr != nil {
				return 0, aerr
			}
			if created {
				return 1, nil
			}
			// Lost the create race: re-read and fall through to the increment path.
			b, err = l.store.Get(ns, key)
		} else {
			if serr := l.store.Set(ns, key, itob(1), ttl); serr != nil {
				return 0, serr
			}
			return 1, nil
		}
	}
	if err != nil {
		return 0, err
	}
	cur := btoi(b)
	if cur >= limit {
		// Already at the limit: reject without another write, so a rejected flood
		// cannot grow the counter without bound.
		return cur + 1, nil
	}
	if serr := l.store.Set(ns, key, itob(cur+1), ttl); serr != nil {
		return 0, serr
	}
	return cur + 1, nil
}

func ttlFor(window time.Duration) time.Duration {
	if ttl := 2 * window; ttl > minTTL {
		return ttl
	}
	return minTTL
}

func itob(n int) []byte { return []byte(strconv.Itoa(n)) }

func btoi(b []byte) int {
	n, _ := strconv.Atoi(string(b))
	return n
}
