package shared

import (
	"context"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/kayo/ayatosrc"
	"github.com/Hayao0819/Kamisato/kayo/federate"
	"github.com/Hayao0819/Kamisato/kayo/overlay"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

func LoadConfig(cmd *cobra.Command) (*conf.KayoConfig, error) {
	configFile, _ := cmd.Flags().GetString("config")
	return conf.LoadKayoConfig(cmd.Flags(), configFile)
}

// BuildComposite creates and syncs the overlay and ayato sources into an ungated
// Composite, shared by the daemon and the verify command. The overlay Registry is
// returned separately (nil when no overlays are configured) so the daemon can
// serve approved overlay pins from its checkouts; it is already registered on the
// Composite.
func BuildComposite(ctx context.Context, cfg *conf.KayoConfig) (*federate.Composite, *overlay.Registry, error) {
	comp := federate.New()
	var reg *overlay.Registry
	if len(cfg.Overlays) > 0 {
		reg = overlay.New(cfg.ResolvedCacheDir(), cfg.Overlays)
		slog.Info("Syncing overlays", "count", len(cfg.Overlays), "cache", cfg.ResolvedCacheDir())
		if err := reg.Sync(ctx); err != nil {
			return nil, nil, errors.WrapErr(err, "initial overlay sync failed")
		}
		comp.Add(reg, federate.TierOverlay, 0, "overlay")
	}
	if len(cfg.Ayato) > 0 {
		pins, err := ayatosrc.OpenPinStore(cfg.AyatoPinStorePath())
		if err != nil {
			return nil, nil, errors.WrapErr(err, "failed to open ayato pin store")
		}
		for _, a := range cfg.Ayato {
			src, err := ayatosrc.New(ayatosrc.Options{
				Name:            a.Name,
				BaseURL:         a.URL,
				PubKey:          a.PubKey,
				MaxAge:          a.ResolvedMaxAge(),
				Insecure:        a.Insecure,
				TrustOnFirstUse: a.TrustOnFirstUse,
				Pins:            pins,
			})
			if err != nil {
				return nil, nil, errors.WrapErr(err, "ayato source "+a.Name)
			}
			if err := src.Sync(ctx); err != nil {
				slog.Error("ayato source initial sync failed", "name", a.Name, "error", err)
			}
			if a.Delegated() {
				comp.AddDelegated(src, federate.TierAyato, a.Priority, a.Name, src.Verified)
			} else {
				comp.Add(src, federate.TierAyato, a.Priority, a.Name)
			}
			slog.Info("ayato source added", "name", a.Name, "url", a.URL, "priority", a.Priority,
				"delegated", a.Delegated(), "insecure", a.Insecure)
		}
	}
	return comp, reg, nil
}

func UpstreamClient(cfg *conf.KayoConfig) *aurweb.AURUpstream {
	if !cfg.Upstream.Enabled {
		return nil
	}
	return aurweb.NewAURUpstream(cfg.Upstream.RPCURL,
		aurweb.WithGitBase(cfg.Upstream.GitBase),
		aurweb.WithUserAgent(cfg.Upstream.UserAgent),
	)
}
