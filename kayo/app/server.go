package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/internal/errors"
	httpserver "github.com/Hayao0819/Kamisato/internal/http/server"
	kayoconfig "github.com/Hayao0819/Kamisato/kayo/config"
	"github.com/Hayao0819/Kamisato/kayo/federate"
	"github.com/Hayao0819/Kamisato/kayo/gitserve"
	"github.com/Hayao0819/Kamisato/kayo/trust"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

func Run(ctx context.Context, cfg *kayoconfig.KayoConfig) error {
	store, err := trust.Open(cfg.ResolvedTrustStore())
	if err != nil {
		return errors.WrapErr(err, "failed to open trust store")
	}
	mode := cfg.ResolvedEnforceMode()

	composite, overlays, err := BuildComposite(ctx, cfg)
	if err != nil {
		return err
	}
	composite.SetGate(store, mode)

	if overlays != nil {
		count, materializeErr := gitserve.MaterializePins(ctx, cfg.ServedRoot(), overlays.SourceDirs(), func(pkgbase string) (string, bool) {
			approval, ok := store.Approval(pkgbase)
			return approval.Commit, ok
		})
		if materializeErr != nil {
			slog.Warn("some overlay pins could not be materialized", "error", materializeErr)
		}
		if count > 0 {
			slog.Info("served approved overlay pins", "count", count)
		}
	}

	opts := []aurweb.Option{aurweb.WithLogger(slog.Default())}
	if upstream := UpstreamClient(cfg); upstream != nil {
		opts = append(opts, aurweb.WithUpstream(&federate.TrustUpstream{AURUpstream: upstream, Store: store, Mode: mode}))
		slog.Info("Upstream AUR fallback enabled", "git_base", upstream.GitBase(), "enforce_mode", mode)
	} else {
		slog.Warn("Upstream AUR fallback disabled; only overlay and ayato packages resolve")
	}
	surface := aurweb.New(composite, opts...)

	if cfg.RefreshMinutes > 0 {
		go refreshLoop(ctx, composite, time.Duration(cfg.RefreshMinutes)*time.Minute)
	}

	engine := httpserver.NewEngine()
	engine.GET("/health", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	engine.NoRoute(gin.WrapH(gitserve.NewHandler(cfg.ServedRoot(), surface)))

	server := httpserver.NewServer(cfg.ListenAddr(), engine)
	slog.Info("kayo listening", "addr", cfg.ListenAddr())
	return httpserver.ServeHTTP(ctx, server, nil)
}

func refreshLoop(ctx context.Context, composite *federate.Composite, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := composite.Sync(ctx); err != nil {
				slog.Error("source refresh failed", "error", err)
			}
		}
	}
}
