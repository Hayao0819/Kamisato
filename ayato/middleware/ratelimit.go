package middleware

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
)

// WithRateLimiter wires the shared kv-backed limiter so limits hold across
// replicas. An unwired Middleware leaves RateLimit as a pass-through.
func (m *Middleware) WithRateLimiter(store kv.Store) *Middleware {
	m.limiter = NewRateLimiter(store, kv.ErrNotFound)
	return m
}

// RateLimit returns a per-client-IP middleware that answers excess requests with
// 429 and a Retry-After header, backed by the shared kv limiter (wired via
// WithRateLimiter) so the limit holds across Cloud Run replicas; unwired it is a
// pass-through. The (rate r, burst) token-bucket params are mapped to an
// equivalent fixed window (see fixedWindow), and each call site gets a distinct
// scope so independent route limiters keep independent counters.
func (m *Middleware) RateLimit(r rate.Limit, burst int) gin.HandlerFunc {
	limit, window := fixedWindow(r, burst)
	scope := "mw" + strconv.FormatInt(m.rlScope.Add(1), 10)

	return func(c *gin.Context) {
		if m.limiter == nil {
			c.Next()
			return
		}
		allowed, retry := m.limiter.Allow(scope, c.ClientIP(), limit, window)
		if allowed {
			c.Next()
			return
		}
		c.Header("Retry-After", retryAfterValue(retry))
		c.AbortWithStatus(http.StatusTooManyRequests)
	}
}

func retryAfterValue(retry time.Duration) string {
	seconds := int64(math.Ceil(retry.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	return strconv.FormatInt(seconds, 10)
}

// fixedWindow maps a token-bucket (rate r tokens/sec, burst) to a fixed window of
// `burst` requests per burst/r seconds. A non-positive or infinite rate means no
// limit.
func fixedWindow(r rate.Limit, burst int) (int, time.Duration) {
	if burst <= 0 || r <= 0 || r >= rate.Inf {
		return 0, 0
	}
	window := time.Duration(float64(burst) / float64(r) * float64(time.Second))
	if window <= 0 {
		window = time.Second
	}
	return burst, window
}
