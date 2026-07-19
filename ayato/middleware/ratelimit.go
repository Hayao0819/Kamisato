package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/Hayao0819/Kamisato/ayato/platform"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	sharedlimit "github.com/Hayao0819/Kamisato/pkg/ratelimit"
)

// WithRateLimiter wires the shared kv-backed limiter so limits hold across
// replicas. An unwired Middleware leaves RateLimit as a pass-through.
func (m *Middleware) WithRateLimiter(store kv.Store) *Middleware {
	m.limiter = platform.NewRateLimiter(store, kv.ErrNotFound)
	return m
}

// RateLimit returns a per-client-IP middleware that answers excess requests with
// 429 and a Retry-After header, backed by the shared kv limiter (wired via
// WithRateLimiter) so the limit holds across Cloud Run replicas; unwired it is a
// pass-through. The (rate r, burst) token-bucket params are mapped to an
// equivalent fixed window (see fixedWindow), and each call site gets a distinct
// scope so independent route limiters keep independent counters.
func (m *Middleware) RateLimit(r rate.Limit, burst int) gin.HandlerFunc {
	policy := fixedWindow(r, burst)
	scope := "mw" + strconv.FormatInt(m.rlScope.Add(1), 10)

	return func(c *gin.Context) {
		if m.limiter == nil {
			c.Next()
			return
		}
		decision := m.limiter.Allow(scope, c.ClientIP(), policy)
		if decision.Allowed {
			c.Next()
			return
		}
		c.Header("Retry-After", sharedlimit.RetryAfterValue(decision.RetryAfter))
		c.AbortWithStatus(http.StatusTooManyRequests)
	}
}

// fixedWindow maps a token-bucket (rate r tokens/sec, burst) to a fixed window of
// `burst` requests per burst/r seconds. A non-positive or infinite rate means no
// limit.
func fixedWindow(r rate.Limit, burst int) sharedlimit.Policy {
	if burst <= 0 || r <= 0 || r >= rate.Inf {
		return sharedlimit.Policy{}
	}
	window := time.Duration(float64(burst) / float64(r) * float64(time.Second))
	if window <= 0 {
		window = time.Second
	}
	return sharedlimit.Policy{Limit: burst, Window: window}
}
