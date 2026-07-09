package aur

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/ratelimit"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

// aurRPCLimiter adapts the shared kv limiter to aurweb's per-client Limiter so the
// throttle holds across replicas.
type aurRPCLimiter struct {
	lim    *ratelimit.Limiter
	limit  int
	window time.Duration
}

func (a aurRPCLimiter) Allow(client string) bool {
	ok, _ := a.lim.Allow("aur:rpc", client, a.limit, a.window)
	return ok
}

// Module is the assembled AUR wiring: Server is the read-only aurweb surface
// (/rpc, git redirects) mounted as the NoRoute fallback, and Service is the
// gin-free backend for the admin/catalog surface.
type Module struct {
	Server  http.Handler
	Service *Service
}

// New builds the AUR module from config and the shared KV store.
func New(cfg *conf.AyatoConfig, store kv.Store) (*Module, error) {
	backend := NewBackend(store, cfg.AUR.Maintainer)

	opts := []aurweb.Option{aurweb.WithLogger(slog.Default())}
	if cfg.AUR.Upstream.Enabled {
		up := aurweb.NewAURUpstream(cfg.AUR.Upstream.RPCURL,
			aurweb.WithGitBase(cfg.AUR.Upstream.GitBase),
			aurweb.WithUserAgent(cfg.AUR.Upstream.UserAgent),
		)
		opts = append(opts, aurweb.WithUpstream(up))
	}
	// The raw NoRoute handler bypasses gin's trusted-proxy ClientIP(), so the limiter
	// keys on the real peer; the shared-kv counter holds the daily limit across replicas.
	rateLimit := aurweb.DefaultRateLimit
	if cfg.AUR.RateLimitPerDay != nil {
		rateLimit = *cfg.AUR.RateLimitPerDay
	}
	if rateLimit > 0 {
		lim := aurRPCLimiter{lim: ratelimit.New(store), limit: rateLimit, window: aurweb.DefaultRateWindow}
		opts = append(opts, aurweb.WithLimiter(lim, nil))
	}

	// TTL bounds both the signed envelope's freshness and how long the public
	// /catalog response is cached.
	ttl := time.Duration(cfg.AUR.CatalogTTLMinutes) * time.Minute
	if ttl <= 0 {
		ttl = 60 * time.Minute
	}

	signer, err := NewCatalogSignerFromEnv(ttl)
	if err != nil {
		return nil, errors.WrapErr(err, "failed to build catalog signer")
	}
	if signer != nil {
		slog.Info("AUR catalog signing enabled", "key_id", signer.KeyID())
	} else {
		slog.Warn("AYATO_AUR_SIGNING_SEED is unset; the kayo-facing catalog is served unsigned")
	}

	srv := aurweb.New(backend, opts...)
	svc := NewService(backend, ttl).WithSigner(signer)

	slog.Info("aurweb-compatible API enabled",
		"upstream", cfg.AUR.Upstream.Enabled, "signed", signer != nil, "rate_limit_per_day", rateLimit)
	return &Module{Server: srv, Service: svc}, nil
}
