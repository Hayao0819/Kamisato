package cmd

import (
	"context"
	"log/slog"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
	ayatosrc "github.com/Hayao0819/Kamisato/sara/ayato"
	"github.com/Hayao0819/Kamisato/sara/federate"
	"github.com/Hayao0819/Kamisato/sara/overlay"
)

// buildComposite creates and syncs the overlay and ayato sources into an
// ungated Composite, shared by the daemon and the verify command.
func buildComposite(ctx context.Context, cfg *conf.SaraConfig) (*federate.Composite, error) {
	comp := federate.New()
	if len(cfg.Overlays) > 0 {
		reg := overlay.New(cfg.ResolvedCacheDir(), cfg.Overlays)
		slog.Info("Syncing overlays", "count", len(cfg.Overlays), "cache", cfg.ResolvedCacheDir())
		if err := reg.Sync(ctx); err != nil {
			return nil, utils.WrapErr(err, "initial overlay sync failed")
		}
		comp.Add(reg, federate.TierOverlay, 0, "overlay")
	}
	for _, a := range cfg.Ayato {
		src := ayatosrc.New(a.Name, a.URL)
		if err := src.Sync(ctx); err != nil {
			slog.Error("ayato source initial sync failed", "name", a.Name, "error", err)
		}
		comp.Add(src, federate.TierAyato, a.Priority, a.Name)
		slog.Info("ayato source added", "name", a.Name, "url", a.URL, "priority", a.Priority)
	}
	return comp, nil
}

// upstreamClient builds the AUR upstream from config, or nil when disabled.
func upstreamClient(cfg *conf.SaraConfig) *aurweb.AURUpstream {
	if !cfg.Upstream.Enabled {
		return nil
	}
	return aurweb.NewAURUpstream(cfg.Upstream.RPCURL,
		aurweb.WithGitBase(cfg.Upstream.GitBase),
		aurweb.WithUserAgent(cfg.Upstream.UserAgent),
	)
}
