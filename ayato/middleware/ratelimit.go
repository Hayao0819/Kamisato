package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// rlEntry pairs a client's limiter with a last-seen time for the idle sweep.
type rlEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// rateLimiter is the shared per-IP limiter state behind RateLimit. Idle clients
// are swept so the map can't grow unbounded under IP churn.
type rateLimiter struct {
	mu       sync.Mutex
	clients  map[string]*rlEntry
	r        rate.Limit
	burst    int
	sweepRun bool
}

const (
	rlIdleTTL    = 10 * time.Minute
	rlSweepEvery = time.Minute
)

// RateLimit returns a per-IP token-bucket middleware (rate r, given burst) that
// rejects excess requests with 429 and a Retry-After header.
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

// startSweeper launches the idle-eviction loop, at most once.
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
