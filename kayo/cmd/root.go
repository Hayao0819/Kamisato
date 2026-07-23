package cmd

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/ginutil"
	"github.com/Hayao0819/Kamisato/internal/version"
	auditcmd "github.com/Hayao0819/Kamisato/kayo/cmd/audit"
	ayatocmd "github.com/Hayao0819/Kamisato/kayo/cmd/ayato"
	hookcmd "github.com/Hayao0819/Kamisato/kayo/cmd/hook"
	"github.com/Hayao0819/Kamisato/kayo/cmd/shared"
	trustcmd "github.com/Hayao0819/Kamisato/kayo/cmd/trust"
	updatecmd "github.com/Hayao0819/Kamisato/kayo/cmd/update"
	verifycmd "github.com/Hayao0819/Kamisato/kayo/cmd/verify"
	"github.com/Hayao0819/Kamisato/kayo/federate"
	"github.com/Hayao0819/Kamisato/kayo/gitserve"
	"github.com/Hayao0819/Kamisato/kayo/trust"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

func RootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "kayo",
		Short: "Local aurweb-compatible overlay that intervenes in AUR-helper resolution",
		RunE:  run,
	}
	cmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug mode")
	cmd.PersistentFlags().StringP("config", "c", "", "Config file")
	cmd.Flags().String("addr", "", "Listen address (host:port, default 127.0.0.1:10713)")
	cliutil.SetVersion(&cmd)
	cliutil.AddNoColorFlag(&cmd)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.AddCommand(auditcmd.Cmd(), trustcmd.Cmd(), updatecmd.Cmd(), verifycmd.Cmd(), hookcmd.Cmd(), ayatocmd.Cmd(), version.Command())
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

	ginutil.Setup(cmd, cfg.Debug)

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := trust.Open(cfg.ResolvedTrustStore())
	if err != nil {
		return errors.WrapErr(err, "failed to open trust store")
	}
	mode := cfg.ResolvedEnforceMode()

	comp, overlays, err := shared.BuildComposite(ctx, cfg)
	if err != nil {
		return err
	}
	comp.SetGate(store, mode)

	// Serve approved overlay pins from their checkouts so a client cloning through
	// kayo gets the exact reviewed commit, not the overlay's floating HEAD; an
	// overlay pkgbase with no approval falls through to the redirect.
	if overlays != nil {
		n, perr := gitserve.MaterializePins(ctx, cfg.ServedRoot(), overlays.SourceDirs(), func(pkgbase string) (string, bool) {
			ap, ok := store.Approval(pkgbase)
			return ap.Commit, ok
		})
		if perr != nil {
			slog.Warn("some overlay pins could not be materialized", "error", perr)
		}
		if n > 0 {
			slog.Info("served approved overlay pins", "count", n)
		}
	}

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

	engine := ginutil.NewEngine()
	engine.GET("/health", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	// Approved packages are served from their pinned local repo (variant B);
	// everything else (/rpc, unmanaged /<pkgbase>.git redirects, /cgit, dumps)
	// falls through to the aurweb surface.
	engine.NoRoute(gin.WrapH(gitserve.NewHandler(cfg.ServedRoot(), srv)))

	httpSrv := ginutil.NewServer(cfg.ListenAddr(), engine)
	slog.Info("kayo listening", "addr", cfg.ListenAddr())
	return ginutil.ServeHTTP(ctx, httpSrv, nil)
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
