package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// rlEntry is a per-client token-bucket limiter with a last-seen timestamp used
// by the idle sweep to evict stale clients.
type rlEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// rateLimiter is the shared per-IP limiter state behind RateLimit. A background
// ticker evicts clients idle longer than rlIdleTTL so the map cannot grow
// unbounded under churn (e.g. rotating source IPs).
type rateLimiter struct {
	mu       sync.Mutex
	clients  map[string]*rlEntry
	r        rate.Limit
	burst    int
	sweepRun bool
}

const (
	// rlIdleTTL is how long a client may sit idle before its limiter is evicted.
	rlIdleTTL = 10 * time.Minute
	// rlSweepEvery is the eviction sweep interval.
	rlSweepEvery = time.Minute
)

// RateLimit returns a per-IP rate-limiting middleware: each client IP gets its
// own token bucket of rate r with the given burst. Requests over the limit are
// rejected with 429 and a Retry-After header. The limiter map is swept on a
// background ticker so idle clients are evicted and memory stays bounded.
func (m *Middleware) RateLimit(r rate.Limit, burst int) gin.HandlerFunc {
	rl := &rateLimiter{
		clients: make(map[string]*rlEntry),
		r:       r,
		burst:   burst,
	}
	rl.startSweeper()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		limiter := rl.limiterFor(ip)
		if !limiter.Allow() {
			// Suggest a retry window derived from the bucket refill rate.
			retry := 1
			if r > 0 {
				if secs := int(1 / float64(r)); secs > retry {
					retry = secs
				}
			}
			c.Header("Retry-After", strconv.Itoa(retry))
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}
		c.Next()
	}
}

// limiterFor returns the client's limiter, lazily creating it, and refreshes its
// last-seen timestamp.
func (rl *rateLimiter) limiterFor(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	e, ok := rl.clients[ip]
	if !ok {
		e = &rlEntry{limiter: rate.NewLimiter(rl.r, rl.burst)}
		rl.clients[ip] = e
	}
	e.lastSeen = time.Now()
	return e.limiter
}

// startSweeper launches the idle-eviction loop once per limiter.
func (rl *rateLimiter) startSweeper() {
	rl.mu.Lock()
	if rl.sweepRun {
		rl.mu.Unlock()
		return
	}
	rl.sweepRun = true
	rl.mu.Unlock()

	go func() {
		ticker := time.NewTicker(rlSweepEvery)
		defer ticker.Stop()
		for range ticker.C {
			cutoff := time.Now().Add(-rlIdleTTL)
			rl.mu.Lock()
			for ip, e := range rl.clients {
				if e.lastSeen.Before(cutoff) {
					delete(rl.clients, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()
}
