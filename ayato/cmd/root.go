package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/aur"
	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/ciauth"
	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/router"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/conf"
	utils "github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/internal/weblog"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

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

			pkgNameRepo, pkgBinaryRepo, authRepo, kvStore, err := repository.New(cfg)
			if err != nil {
				return utils.WrapErr(err, "failed to initialize repository")
			}
			defer func() { _ = kvStore.Close() }()

			signerRepo := repository.NewSignerRepository(kvStore)
			denylistRepo := repository.NewDenylistRepository(kvStore)
			s := service.New(pkgNameRepo, pkgBinaryRepo, authRepo, signerRepo, cfg)
			h := handler.New(s, cfg)
			m := middleware.New(cfg)

			// The admin allowlist is the only persisted auth state; sessions, CLI
			// tokens, one-time codes, and OAuth state are all stateless-signed.
			if err := s.SeedBootstrapAdmin(cfg.Auth.BootstrapAdminGitHubID); err != nil {
				return utils.WrapErr(err, "failed to seed bootstrap admin")
			}
			if len(cfg.Auth.SessionSecret) > 0 {
				signer, serr := auth.NewSigner(cfg.Auth.SessionSecret)
				if serr != nil {
					return utils.WrapErr(serr, "failed to build session signer")
				}
				h.WithAuth(signer)
				s.WithDenylist(denylistRepo)
				m.WithAuth(s, signer).WithDenylist(denylistRepo)
			} else {
				// No signer: mutating and admin routes fail closed (503) rather than
				// allowing unauthenticated access.
				slog.Warn("authentication is not configured; mutating and admin routes will fail closed (503) until auth.session_secret and auth.github are set")
			}

			// CI publish credentials are independent of the user/admin auth above:
			// a repo can publish via API key or GitHub OIDC without a session secret.
			ci, cierr := ciauth.New(cmd.Context(), cfg.Auth.CI)
			if cierr != nil {
				return utils.WrapErr(cierr, "failed to init CI auth")
			}
			m.WithCIAuth(ci)

			engine := gin.New()
			engine.Use(gin.Recovery())
			engine.Use(weblog.GinLog())

			// Trust no proxy by default so ClientIP() is the real peer and the
			// spoofable X-Forwarded-For is ignored (the rate-limit key is only
			// trustworthy this way); honor XFF only behind trusted_proxies.
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

			if cfg.AUR.Enabled {
				mod, merr := aur.New(cfg, kvStore)
				if merr != nil {
					return utils.WrapErr(merr, "failed to initialize AUR module")
				}
				router.SetAUR(engine, m, mod.Server, handler.NewAURHandler(mod.Service))
			}

			if err := s.InitAll(); err != nil {
				return utils.WrapErr(err, "failed to initialize services")
			}
			slog.Info("All services initialized")

			slog.Info("Waiting on port", "port", cfg.Port)
			// ReadHeaderTimeout bounds slow-header (slowloris) attacks; IdleTimeout
			// reaps idle keep-alives. No ReadTimeout or WriteTimeout: large package
			// uploads (read body) and /repo downloads + proxied miko SSE (write) need
			// unbounded durations; the upload handlers cap the body size with
			// http.MaxBytesReader instead.
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
	cmd.AddCommand(aurCmd())

	return &cmd
}
