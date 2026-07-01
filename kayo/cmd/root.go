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
	"github.com/Hayao0819/Kamisato/internal/weblog"
	ayatocmd "github.com/Hayao0819/Kamisato/kayo/cmd/ayato"
	hookcmd "github.com/Hayao0819/Kamisato/kayo/cmd/hook"
	"github.com/Hayao0819/Kamisato/kayo/cmd/shared"
	trustcmd "github.com/Hayao0819/Kamisato/kayo/cmd/trust"
	"github.com/Hayao0819/Kamisato/kayo/federate"
	"github.com/Hayao0819/Kamisato/kayo/gitserve"
	"github.com/Hayao0819/Kamisato/kayo/trust"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

func RootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "kayo",
		Short: "Local aurweb-compatible overlay that intervenes in AUR-helper resolution",
		RunE:  run,
	}
	cmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug mode")
	cmd.PersistentFlags().StringP("config", "c", "", "Config file")
	cmd.Flags().IntP("port", "p", 0, "Listen port (default 10713)")
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.AddCommand(auditCmd(), trustcmd.Cmd(), updateCmd(), verifyCmd(), hookcmd.Cmd(), ayatocmd.Cmd())
	return &cmd
}

func run(cmd *cobra.Command, _ []string) error {
	configFile, err := cmd.Flags().GetString("config")
	if err != nil {
		return err
	}

	cfg, err := conf.LoadKayoConfig(cmd.Flags(), configFile)
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

	comp, err := shared.BuildComposite(ctx, cfg)
	if err != nil {
		return err
	}
	comp.SetGate(store, mode)

	opts := []aurweb.Option{aurweb.WithLogger(slog.Default())}
	if up := shared.UpstreamClient(cfg); up != nil {
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
	engine.Use(gin.Recovery(), weblog.GinLog())
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
		slog.Info("kayo listening", "addr", cfg.ListenAddr())
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
