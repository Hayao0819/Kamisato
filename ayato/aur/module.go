package aur

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

// Module is the assembled AUR wiring: Server is the read-only aurweb surface
// (/rpc, git redirects) mounted as the NoRoute fallback, and Handler is the
// admin/catalog surface.
type Module struct {
	Server  http.Handler
	Handler *Handler
}

// New builds the AUR module from config and the shared KV store: the backend, the
// aurweb Server (optional upstream fallback and per-client /rpc rate limiting), the
// catalog TTL, and the catalog signer loaded from AYATO_AUR_SIGNING_SEED.
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
	// The limiter keys on the request's real peer: the aurweb Server is mounted as a
	// raw handler, so gin's trusted-proxy ClientIP() resolution does not reach it.
	rateLimit := aurweb.DefaultRateLimit
	if cfg.AUR.RateLimitPerDay != nil {
		rateLimit = *cfg.AUR.RateLimitPerDay
	}
	if rateLimit > 0 {
		opts = append(opts, aurweb.WithRateLimit(rateLimit, aurweb.DefaultRateWindow, nil))
	}

	// TTL bounds both the signed envelope's freshness and how long the public
	// /catalog response is cached.
	ttl := time.Duration(cfg.AUR.CatalogTTLMinutes) * time.Minute
	if ttl <= 0 {
		ttl = 60 * time.Minute
	}

	signer, err := NewCatalogSignerFromEnv(ttl)
	if err != nil {
		return nil, utils.WrapErr(err, "failed to build catalog signer")
	}
	if signer != nil {
		slog.Info("AUR catalog signing enabled", "key_id", signer.KeyID())
	} else {
		slog.Warn("AYATO_AUR_SIGNING_SEED is unset; the kayo-facing catalog is served unsigned")
	}

	srv := aurweb.New(backend, opts...)
	handler := NewHandler(backend, ttl).WithSigner(signer)

	slog.Info("aurweb-compatible API enabled",
		"upstream", cfg.AUR.Upstream.Enabled, "signed", signer != nil, "rate_limit_per_day", rateLimit)
	return &Module{Server: srv, Handler: handler}, nil
}
