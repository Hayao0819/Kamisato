package aurweb

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"
)

// DefaultRateLimit and DefaultRateWindow mirror aurweb's RPC limit of 4000
// requests per day per client.
const (
	DefaultRateLimit  = 4000
	DefaultRateWindow = 24 * time.Hour
)

// maxRateBuckets bounds the limiter's memory; expired buckets are swept when exceeded.
const maxRateBuckets = 100_000

// WithRateLimit enables per-client RPC rate limiting (at most n requests per window, HTTP 429 when exceeded;
// n<=0 is a no-op; nil keyFn keys on remote IP). The counter is in-memory and per-instance (not shared across
// replicas); for cross-replica limits use WithLimiter.
func WithRateLimit(n int, window time.Duration, keyFn func(*http.Request) string) Option {
	return func(s *Server) {
		if n > 0 && window > 0 {
			if keyFn == nil {
				keyFn = remoteIP
			}
			s.limiter = newRateLimiter(n, window)
			s.limiterFn = keyFn
		}
	}
}

type rateLimiter struct {
	n      int
	window time.Duration

	mu   sync.Mutex
	hits map[string]*rateBucket
}

type rateBucket struct {
	count int
	start time.Time
}

func newRateLimiter(n int, window time.Duration) *rateLimiter {
	return &rateLimiter{n: n, window: window, hits: make(map[string]*rateBucket)}
}

// Allow records a request for key and reports whether it is within the limit.
func (l *rateLimiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	b := l.hits[key]
	if b == nil || now.Sub(b.start) >= l.window {
		if len(l.hits) >= maxRateBuckets {
			// Expired buckets first; if a distinct-key flood leaves every bucket
			// in-window the sweep frees nothing, so evict the oldest to keep the
			// cap hard (a fixed window can't bound memory by expiry alone).
			l.sweepLocked(now)
			if len(l.hits) >= maxRateBuckets {
				l.evictOldestLocked()
			}
		}
		l.hits[key] = &rateBucket{count: 1, start: now}
		return true
	}
	if b.count >= l.n {
		return false
	}
	b.count++
	return true
}

func (l *rateLimiter) sweepLocked(now time.Time) {
	for k, b := range l.hits {
		if now.Sub(b.start) >= l.window {
			delete(l.hits, k)
		}
	}
}

func (l *rateLimiter) evictOldestLocked() {
	var oldestKey string
	var oldest time.Time
	for k, b := range l.hits {
		if oldestKey == "" || b.start.Before(oldest) {
			oldestKey, oldest = k, b.start
		}
	}
	if oldestKey != "" {
		delete(l.hits, oldestKey)
	}
}

func remoteIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// writeRateLimited answers an over-limit request with HTTP 429 (plain JSON, no JSONP), mirroring aurweb's error envelope.
func (s *Server) writeRateLimited(w http.ResponseWriter, version int) {
	body, _ := json.Marshal(map[string]any{
		"version":     versionOrNull(version),
		"type":        "error",
		"resultcount": 0,
		"results":     []any{},
		"error":       "Rate limit reached",
	})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write(body)
}
