package app

import (
	"log/slog"
	"net/http"
	"time"

	ayatoconfig "github.com/Hayao0819/Kamisato/ayato/config"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/ayato/repository/aurrepo"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/service/aur"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

const aurwebRateLimitScope = "aurweb:rpc"

// buildAUR assembles the AUR wiring from config and the shared KV store: the
// read-only aurweb surface (/rpc, git redirects) mounted as the NoRoute fallback,
// and the gin-free source-management/catalog service. Composition lives here rather
// than in a feature package so the backend and service stay in the standard
// repository/service layers.
func buildAUR(cfg *ayatoconfig.AyatoConfig, store kv.Store) (http.Handler, *aur.Service, error) {
	backend := aurrepo.NewBackend(store, cfg.AUR.Maintainer)

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
		limiter := middleware.NewRateLimiter(store, kv.ErrNotFound)
		opts = append(opts, aurweb.WithRateLimiter(func(client string) (bool, time.Duration) {
			return limiter.Allow(aurwebRateLimitScope, client, rateLimit, aurweb.DefaultRateWindow)
		}, nil))
	}

	// TTL bounds both the signed envelope's freshness and how long the public
	// /catalog response is cached.
	ttl := time.Duration(cfg.AUR.CatalogTTLMinutes) * time.Minute
	if ttl <= 0 {
		ttl = 60 * time.Minute
	}

	signer, err := aur.NewCatalogSignerFromEnv(ttl)
	if err != nil {
		return nil, nil, errors.WrapErr(err, "failed to build catalog signer")
	}
	if signer != nil {
		slog.Info("AUR catalog signing enabled", "key_id", signer.KeyID())
	} else {
		slog.Warn("AYATO_AUR_SIGNING_SEED is unset; the kayo-facing catalog is served unsigned")
	}

	srv := aurweb.New(backend, opts...)
	svc := aur.NewService(backend, ttl).WithSigner(signer)

	slog.Info("aurweb-compatible API enabled",
		"upstream", cfg.AUR.Upstream.Enabled, "signed", signer != nil, "rate_limit_per_day", rateLimit)
	return srv, svc, nil
}
