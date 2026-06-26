package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/router"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/conf"
	utils "github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

// RootCmd returns the root command for Ayato CLI.
func RootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use: "ayato",
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}

			cfg, err := conf.LoadAyatoConfig(cmd.Flags(), configFile)
			if err != nil {
				return err
			}

			if configFile != "" {
				slog.Info("Loaded from config file", "path", configFile)
			}

			if cfg.Debug {
				utils.UseColorLog(slog.LevelDebug)
				slog.Debug("Debug mode enabled")
				gin.SetMode(gin.DebugMode)
			} else {
				utils.UseColorLog(slog.LevelInfo)
				gin.SetMode(gin.ReleaseMode)
			}

			slog.Debug("Configuration loaded", "port", cfg.Port, "debug", cfg.Debug, "repos", cfg.Repos, "maxsize", cfg.MaxSize, "dbtype", cfg.Store.DBType, "storagetype", cfg.Store.StorageType)

			pkgNameRepo, pkgBinaryRepo, authRepo, kvCloser, err := repository.New(cfg)
			if err != nil {
				return utils.WrapErr(err, "failed to initialize repository")
			}
			defer func() { _ = kvCloser.Close() }()

			s := service.New(pkgNameRepo, pkgBinaryRepo, authRepo, cfg)
			h := handler.New(s, cfg)
			m := middleware.New(cfg)

			// The admin allowlist is the only persisted auth state; it rides the
			// shared kv store behind the repository layer. Seed the bootstrap admin
			// on first run through the service. Sessions, CLI tokens, one-time
			// codes, and OAuth state are all stateless-signed by the signer.
			if err := s.SeedBootstrapAdmin(cfg.Auth.BootstrapAdminGitHubID); err != nil {
				return utils.WrapErr(err, "failed to seed bootstrap admin")
			}
			if len(cfg.Auth.SessionSecret) > 0 {
				signer, serr := auth.NewSigner(cfg.Auth.SessionSecret)
				if serr != nil {
					return utils.WrapErr(serr, "failed to build session signer")
				}
				h.WithAuth(signer)
				m.WithAuth(s, signer)
			} else {
				// No signer: the mutating and admin routes fail closed (503) rather
				// than allowing unauthenticated access. Configure auth.session_secret
				// (and auth.github) to enable them.
				slog.Warn("authentication is not configured; mutating and admin routes will fail closed (503) until auth.session_secret and auth.github are set")
			}

			engine := gin.New()
			engine.Use(gin.Recovery())
			engine.Use(utils.GinLog())

			// SetTrustedProxies only affects c.ClientIP()/c.RemoteIP() (the
			// X-Forwarded-For chain); it does NOT gate c.GetHeader, so it has no
			// bearing on the OAuth redirect_uri or cookie Secure, which derive from
			// Auth.PublicOrigin. Baseline: trust NO proxy, so ClientIP() returns the
			// real peer and the spoofable X-Forwarded-For is ignored (the rate-limit
			// key is only trustworthy this way). Only when trusted_proxies is set
			// (lumine's CIDR) do we honor XFF from those hops.
			if err := engine.SetTrustedProxies(nil); err != nil {
				return utils.WrapErr(err, "failed to reset trusted proxies")
			}
			if len(cfg.Auth.TrustedProxies) > 0 {
				if err := engine.SetTrustedProxies(cfg.Auth.TrustedProxies); err != nil {
					return utils.WrapErr(err, "failed to set trusted proxies")
				}
			}

			if err := router.SetRoute(engine, h, m); err != nil {
				return utils.WrapErr(err, "failed to set routing")
			}
			slog.Info("Routing initialized")

			if err := s.InitAll(); err != nil {
				return utils.WrapErr(err, "failed to initialize services")
			}
			slog.Info("All services initialized")

			slog.Info("Waiting on port", "port", cfg.Port)
			// ReadHeaderTimeout bounds slow-header (slowloris) attacks; IdleTimeout
			// reaps idle keep-alives. No ReadTimeout or WriteTimeout: large package
			// uploads (read body) and /repo downloads + proxied miko SSE (write) need
			// unbounded durations; body size is bounded by cfg.MaxSize instead.
			srv := &http.Server{
				Addr:              fmt.Sprintf(":%d", cfg.Port),
				Handler:           engine,
				ReadHeaderTimeout: 10 * time.Second,
				IdleTimeout:       120 * time.Second,
				MaxHeaderBytes:    1 << 20,
			}
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
	}
	cmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug mode")
	cmd.PersistentFlags().StringP("config", "c", "", "Config file")
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	return &cmd
}
