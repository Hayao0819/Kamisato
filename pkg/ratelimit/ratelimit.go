// Package ratelimit defines the policy and result shared by in-process and
// distributed fixed-window rate limiters.
package ratelimit

import (
	"math"
	"strconv"
	"sync"
	"time"
)

// DefaultMaxBuckets bounds the number of distinct scope/client pairs retained
// by an in-memory limiter.
const DefaultMaxBuckets = 100_000

// Policy describes a fixed-window request budget. A non-positive field disables
// the policy.
type Policy struct {
	Limit  int
	Window time.Duration
}

// Enabled reports whether the policy should limit requests.
func (p Policy) Enabled() bool {
	return p.Limit > 0 && p.Window > 0
}

// Decision is the result of recording one request.
type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
}

// Limiter records requests by scope and client. Scope keeps independent
// consumers from sharing a budget when they use one limiter.
type Limiter interface {
	Allow(scope, client string, policy Policy) Decision
}

// WindowAt identifies the clock-aligned fixed window containing now and returns
// the time remaining until the next window. Callers must pass a positive window.
func WindowAt(now time.Time, window time.Duration) (int64, time.Duration) {
	nanos := window.Nanoseconds()
	index := now.UnixNano() / nanos
	retry := time.Duration((index+1)*nanos - now.UnixNano())
	return index, retry
}

// RetryAfterValue formats a retry duration for an HTTP Retry-After header.
func RetryAfterValue(retry time.Duration) string {
	seconds := int64(math.Ceil(retry.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	return strconv.FormatInt(seconds, 10)
}

// Memory is a concurrency-safe, bounded in-process fixed-window limiter.
type Memory struct {
	maxBuckets int
	now        func() time.Time

	mu      sync.Mutex
	buckets map[bucketKey]bucket
}

type bucketKey struct {
	scope  string
	client string
}

type bucket struct {
	window    time.Duration
	index     int64
	count     int
	lastSeen  time.Time
	expiresAt time.Time
}

// NewMemory constructs an in-process limiter. A non-positive maximum uses
// DefaultMaxBuckets.
func NewMemory(maxBuckets int) *Memory {
	if maxBuckets <= 0 {
		maxBuckets = DefaultMaxBuckets
	}
	return &Memory{
		maxBuckets: maxBuckets,
		now:        time.Now,
		buckets:    make(map[bucketKey]bucket),
	}
}

// Allow records a request in a clock-aligned fixed window.
func (l *Memory) Allow(scope, client string, policy Policy) Decision {
	if !policy.Enabled() {
		return Decision{Allowed: true}
	}

	now := l.now()
	index, retry := WindowAt(now, policy.Window)
	key := bucketKey{scope: scope, client: client}

	l.mu.Lock()
	defer l.mu.Unlock()

	current, exists := l.buckets[key]
	if !exists || current.window != policy.Window || current.index != index {
		if !exists && len(l.buckets) >= l.maxBuckets {
			l.sweepLocked(now)
			if len(l.buckets) >= l.maxBuckets {
				l.evictOldestLocked()
			}
		}
		l.buckets[key] = bucket{
			window:    policy.Window,
			index:     index,
			count:     1,
			lastSeen:  now,
			expiresAt: now.Add(retry),
		}
		return Decision{Allowed: true}
	}

	current.lastSeen = now
	if current.count >= policy.Limit {
		l.buckets[key] = current
		return Decision{RetryAfter: retry}
	}
	current.count++
	l.buckets[key] = current
	return Decision{Allowed: true}
}

func (l *Memory) sweepLocked(now time.Time) {
	for key, entry := range l.buckets {
		if !entry.expiresAt.After(now) {
			delete(l.buckets, key)
		}
	}
}

func (l *Memory) evictOldestLocked() {
	var oldestKey bucketKey
	var oldest time.Time
	found := false
	for key, entry := range l.buckets {
		if !found || entry.lastSeen.Before(oldest) {
			oldestKey, oldest, found = key, entry.lastSeen, true
		}
	}
	if found {
		delete(l.buckets, oldestKey)
	}
}
