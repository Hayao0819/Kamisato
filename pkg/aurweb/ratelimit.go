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

// maxRateBuckets bounds the limiter's memory; once exceeded, expired buckets are
// swept before inserting a new one.
const maxRateBuckets = 100_000

// WithRateLimit enables aurweb-style RPC rate limiting: at most n requests per
// window per client, answered with HTTP 429 once exceeded. Disabled by default
// (n<=0 is a no-op). keyFn identifies the client; nil uses the request's remote
// IP. Behind a trusted proxy, pass a keyFn that reads the forwarded address.
//
// The counter is in-memory and per-instance — best-effort, like aurweb's, and
// not shared across replicas; it resets on restart.
func WithRateLimit(n int, window time.Duration, keyFn func(*http.Request) string) Option {
	return func(s *Server) {
		if n > 0 && window > 0 {
			s.limiter = newRateLimiter(n, window, keyFn)
		}
	}
}

type rateLimiter struct {
	n      int
	window time.Duration
	keyFn  func(*http.Request) string

	mu   sync.Mutex
	hits map[string]*rateBucket
}

type rateBucket struct {
	count int
	start time.Time
}

func newRateLimiter(n int, window time.Duration, keyFn func(*http.Request) string) *rateLimiter {
	if keyFn == nil {
		keyFn = remoteIP
	}
	return &rateLimiter{n: n, window: window, keyFn: keyFn, hits: make(map[string]*rateBucket)}
}

func (l *rateLimiter) key(r *http.Request) string { return l.keyFn(r) }

// allow records a request for key and reports whether it is within the limit.
func (l *rateLimiter) allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	b := l.hits[key]
	if b == nil || now.Sub(b.start) >= l.window {
		if len(l.hits) >= maxRateBuckets {
			l.sweepLocked(now)
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

func remoteIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// writeRateLimited answers an over-limit request the way aurweb does: HTTP 429
// with the error envelope (plain JSON, no JSONP wrapping).
func (s *Server) writeRateLimited(w http.ResponseWriter, version int) {
	if version == 0 {
		version = Version
	}
	body, _ := json.Marshal(map[string]any{
		"version":     version,
		"type":        "error",
		"resultcount": 0,
		"results":     []any{},
		"error":       "Rate limit reached",
	})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write(body)
}
