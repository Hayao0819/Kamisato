package aurweb

import (
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/Hayao0819/Kamisato/pkg/ratelimit"
)

// DefaultRateLimit and DefaultRateWindow mirror aurweb's RPC limit of 4000
// requests per day per client.
const (
	DefaultRateLimit  = 4000
	DefaultRateWindow = 24 * time.Hour
)

const rpcRateScope = "aurweb:rpc"

// WithRateLimit enables per-client RPC rate limiting (at most n requests per window, HTTP 429 when exceeded;
// n<=0 is a no-op; nil keyFn keys on remote IP). The counter is in-memory and per-instance (not shared across
// replicas); for cross-replica limits use WithLimiter.
func WithRateLimit(n int, window time.Duration, keyFn func(*http.Request) string) Option {
	return WithLimiter(
		ratelimit.NewMemory(ratelimit.DefaultMaxBuckets),
		ratelimit.Policy{Limit: n, Window: window},
		keyFn,
	)
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
	w.Header().Set("Retry-After", ratelimit.RetryAfterValue(retry))
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write(body)
}
