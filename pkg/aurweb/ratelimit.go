package aurweb

import (
	"encoding/json"
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// DefaultRateLimit and DefaultRateWindow mirror aurweb's RPC limit of 4000
// requests per day per client.
const (
	DefaultRateLimit  = 4000
	DefaultRateWindow = 24 * time.Hour
)

const defaultMaxRateLimitClients = 100_000

// RateLimitFunc decides whether a client may call the RPC endpoint.
type RateLimitFunc func(client string) (allowed bool, retryAfter time.Duration)

// WithRateLimit applies an in-memory per-instance RPC limit. Use
// WithRateLimiter when counters must be shared across replicas.
func WithRateLimit(n int, window time.Duration, keyFn func(*http.Request) string) Option {
	if n <= 0 || window <= 0 {
		return func(*Server) {}
	}
	limiter := newMemoryRateLimiter(defaultMaxRateLimitClients)
	return WithRateLimiter(func(client string) (bool, time.Duration) {
		return limiter.allow(client, n, window)
	}, keyFn)
}

type memoryRateLimiter struct {
	maxClients int
	now        func() time.Time

	mu      sync.Mutex
	buckets map[string]rateLimitBucket
}

type rateLimitBucket struct {
	window    time.Duration
	index     int64
	count     int
	lastSeen  time.Time
	expiresAt time.Time
}

func newMemoryRateLimiter(maxClients int) *memoryRateLimiter {
	if maxClients <= 0 {
		maxClients = defaultMaxRateLimitClients
	}
	return &memoryRateLimiter{
		maxClients: maxClients,
		now:        time.Now,
		buckets:    make(map[string]rateLimitBucket),
	}
}

func (limiter *memoryRateLimiter) allow(client string, limit int, window time.Duration) (bool, time.Duration) {
	if limit <= 0 || window <= 0 {
		return true, 0
	}

	now := limiter.now()
	index, retry := rateLimitWindow(now, window)
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	current, exists := limiter.buckets[client]
	if !exists || current.window != window || current.index != index {
		if !exists && len(limiter.buckets) >= limiter.maxClients {
			limiter.sweep(now)
			if len(limiter.buckets) >= limiter.maxClients {
				limiter.evictOldest()
			}
		}
		limiter.buckets[client] = rateLimitBucket{
			window:    window,
			index:     index,
			count:     1,
			lastSeen:  now,
			expiresAt: now.Add(retry),
		}
		return true, 0
	}

	current.lastSeen = now
	if current.count >= limit {
		limiter.buckets[client] = current
		return false, retry
	}
	current.count++
	limiter.buckets[client] = current
	return true, 0
}

func (limiter *memoryRateLimiter) sweep(now time.Time) {
	for client, entry := range limiter.buckets {
		if !entry.expiresAt.After(now) {
			delete(limiter.buckets, client)
		}
	}
}

func (limiter *memoryRateLimiter) evictOldest() {
	var oldestClient string
	var oldest time.Time
	found := false
	for client, entry := range limiter.buckets {
		if !found || entry.lastSeen.Before(oldest) {
			oldestClient, oldest, found = client, entry.lastSeen, true
		}
	}
	if found {
		delete(limiter.buckets, oldestClient)
	}
}

func rateLimitWindow(now time.Time, window time.Duration) (int64, time.Duration) {
	nanos := window.Nanoseconds()
	index := now.UnixNano() / nanos
	retry := time.Duration((index+1)*nanos - now.UnixNano())
	return index, retry
}

func retryAfterValue(retry time.Duration) string {
	seconds := int64(math.Ceil(retry.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	return strconv.FormatInt(seconds, 10)
}

func remoteIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// writeRateLimited answers an over-limit request with HTTP 429 (plain JSON, no
// JSONP), mirroring aurweb's error envelope.
func (s *Server) writeRateLimited(w http.ResponseWriter, version int, retry time.Duration) {
	body, _ := json.Marshal(map[string]any{
		"version":     versionOrNull(version),
		"type":        "error",
		"resultcount": 0,
		"results":     []any{},
		"error":       "Rate limit reached",
	})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Retry-After", retryAfterValue(retry))
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write(body)
}
