package cmd

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
	ayatosrc "github.com/Hayao0819/Kamisato/sara/ayato"
	"github.com/Hayao0819/Kamisato/sara/federate"
	"github.com/Hayao0819/Kamisato/sara/gitserve"
	"github.com/Hayao0819/Kamisato/sara/overlay"
	"github.com/Hayao0819/Kamisato/sara/trust"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

// RootCmd returns the root command for the sara local AUR overlay daemon.
func RootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "sara",
		Short: "Local aurweb-compatible overlay that intervenes in AUR-helper resolution",
		RunE:  run,
	}
	cmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug mode")
	cmd.PersistentFlags().StringP("config", "c", "", "Config file")
	cmd.Flags().IntP("port", "p", 0, "Listen port (default 10713)")
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.AddCommand(auditCmd(), trustCmd(), updateCmd())
	return &cmd
}

func run(cmd *cobra.Command, _ []string) error {
	configFile, err := cmd.Flags().GetString("config")
	if err != nil {
		return err
	}

	cfg, err := conf.LoadSaraConfig(cmd.Flags(), configFile)
	if err != nil {
		return err
	}

	if cfg.Debug {
		utils.UseColorLog(slog.LevelDebug)
		gin.SetMode(gin.DebugMode)
	} else {
		utils.UseColorLog(slog.LevelInfo)
		gin.SetMode(gin.ReleaseMode)
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := trust.Open(cfg.ResolvedTrustStore())
	if err != nil {
		return utils.WrapErr(err, "failed to open trust store")
	}
	mode := cfg.ResolvedEnforceMode()

	comp := federate.New()
	comp.SetGate(store, mode)
	if len(cfg.Overlays) > 0 {
		reg := overlay.New(cfg.ResolvedCacheDir(), cfg.Overlays)
		slog.Info("Syncing overlays", "count", len(cfg.Overlays), "cache", cfg.ResolvedCacheDir())
		if err := reg.Sync(ctx); err != nil {
			return utils.WrapErr(err, "initial overlay sync failed")
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

	opts := []aurweb.Option{aurweb.WithLogger(slog.Default())}
	if cfg.Upstream.Enabled {
		up := aurweb.NewAURUpstream(cfg.Upstream.RPCURL,
			aurweb.WithGitBase(cfg.Upstream.GitBase),
			aurweb.WithUserAgent(cfg.Upstream.UserAgent),
		)
		// Gate upstream-AUR results through the same trust store (source "aur").
		opts = append(opts, aurweb.WithUpstream(&federate.TrustUpstream{AURUpstream: up, Store: store, Mode: mode}))
		slog.Info("Upstream AUR fallback enabled", "git_base", up.GitBase(), "enforce_mode", mode)
	} else {
		slog.Warn("Upstream AUR fallback disabled; only overlay and ayato packages resolve")
	}
	srv := aurweb.New(comp, opts...)

	if cfg.RefreshMinutes > 0 {
		go refreshLoop(ctx, comp, time.Duration(cfg.RefreshMinutes)*time.Minute)
	}

	engine := gin.New()
	engine.Use(gin.Recovery(), utils.GinLog())
	if err := engine.SetTrustedProxies(nil); err != nil {
		return utils.WrapErr(err, "failed to reset trusted proxies")
	}
	engine.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	// Approved packages are served from their pinned local repo (variant B);
	// everything else (/rpc, unmanaged /<pkgbase>.git redirects, /cgit, dumps)
	// falls through to the aurweb surface.
	engine.NoRoute(gin.WrapH(gitserve.NewHandler(cfg.ServedRoot(), srv)))

	httpSrv := &http.Server{
		Addr:              cfg.ListenAddr(),
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	go func() {
		slog.Info("sara listening", "addr", cfg.ListenAddr())
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpSrv.Shutdown(shutdownCtx)
}

func refreshLoop(ctx context.Context, comp *federate.Composite, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := comp.Sync(ctx); err != nil {
				slog.Error("source refresh failed", "error", err)
			}
		}
	}
}
