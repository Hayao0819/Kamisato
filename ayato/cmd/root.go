package cmd

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	sloggin "github.com/samber/slog-gin"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/ayato/migrate"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/router"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/version"
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
				cliutil.Setup(slog.LevelDebug, cliutil.ColorEnabled(cmd))
				slog.Debug("Debug mode enabled")
				gin.SetMode(gin.DebugMode)
			} else {
				cliutil.Setup(slog.LevelInfo, cliutil.ColorEnabled(cmd))
				gin.SetMode(gin.ReleaseMode)
			}

			slog.Debug("Configuration loaded", "port", cfg.Port, "debug", cfg.Debug, "repos", cfg.Repos, "maxsize", cfg.MaxSize, "dbtype", cfg.Store.DBType, "storagetype", cfg.Store.StorageType)

			pkgNameRepo, pkgBinaryRepo, authRepo, kvStore, err := repository.New(cfg)
			if err != nil {
				return errors.WrapErr(err, "failed to initialize repository")
			}
			defer func() { _ = kvStore.Close() }()

			// Advisory: warn on a layout mismatch but keep serving, so a rollout or an
			// in-progress migration never brings the fleet down.
			if v, inRange, gerr := migrate.Guard(kvStore, migrate.SupportedMin, migrate.SupportedMax); gerr != nil {
				slog.Warn("could not read layout version", "err", gerr)
			} else if !inRange {
				slog.Warn("stored layout version is outside this binary's supported range", "version", v, "min", migrate.SupportedMin, "max", migrate.SupportedMax)
			}

			signerRepo := repository.NewSignerRepository(kvStore)
			denylistRepo := repository.NewDenylistRepository(kvStore)
			replayGuard := repository.NewReplayGuard(kvStore)
			logTokenRepo := repository.NewLogTokenRepository(kvStore)
			deviceRepo := repository.NewDeviceRepository(kvStore)
			s := service.New(pkgNameRepo, pkgBinaryRepo, authRepo, signerRepo, cfg)
			h := handler.New(s, cfg).WithLogTokens(logTokenRepo)
			m := middleware.New(cfg).WithLogTokens(logTokenRepo).WithRateLimiter(kvStore)

			// The admin allowlist is the only persisted auth state; sessions, CLI
			// tokens, one-time codes, and OAuth state are all stateless-signed.
			if err := s.SeedBootstrapAdmin(cfg.Auth.BootstrapAdminGitHubID); err != nil {
				return errors.WrapErr(err, "failed to seed bootstrap admin")
			}
			if len(cfg.Auth.SessionSecret) > 0 {
				signer, serr := auth.NewSigner(cfg.Auth.SessionSecret)
				if serr != nil {
					return errors.WrapErr(serr, "failed to build session signer")
				}
				h.WithAuth(signer)
				h.WithReplayGuard(replayGuard)
				h.WithDeviceStore(deviceRepo)
				s.WithDenylist(denylistRepo)
				m.WithAuth(s, signer).WithDenylist(denylistRepo)
			} else {
				// No signer: mutating and admin routes fail closed (503) rather than
				// allowing unauthenticated access.
				slog.Warn("authentication is not configured; mutating and admin routes will fail closed (503) until auth.session_secret and auth.github are set")
			}

			// CI publish credentials are independent of the user/admin auth above:
			// a repo can publish via API key or GitHub OIDC without a session secret.
			ci, cierr := auth.NewCIAuthorizer(cmd.Context(), cfg.Auth.CI)
			if cierr != nil {
				return errors.WrapErr(cierr, "failed to init CI auth")
			}
			m.WithCIAuth(ci)

			engine := gin.New()
			engine.Use(gin.Recovery())
			engine.Use(sloggin.NewWithConfig(slog.Default(), sloggin.Config{DefaultLevel: slog.LevelDebug, HandleGinDebug: true}))
			engine.Use(m.SecurityHeaders())

			// Trust no proxy by default so ClientIP() is the real peer and the
			// spoofable X-Forwarded-For is ignored (the rate-limit key is only
			// trustworthy this way); honor XFF only behind trusted_proxies.
			if err := engine.SetTrustedProxies(nil); err != nil {
				return errors.WrapErr(err, "failed to reset trusted proxies")
			}
			if len(cfg.Auth.TrustedProxies) > 0 {
				if err := engine.SetTrustedProxies(cfg.Auth.TrustedProxies); err != nil {
					return errors.WrapErr(err, "failed to set trusted proxies")
				}
			}

			if err := router.SetRoute(engine, h, m); err != nil {
				return errors.WrapErr(err, "failed to set routing")
			}
			slog.Info("Routing initialized")

			if cfg.AUR.Enabled {
				aurServer, aurSvc, merr := buildAUR(cfg, kvStore)
				if merr != nil {
					return errors.WrapErr(merr, "failed to initialize AUR module")
				}
				router.SetAUR(engine, m, aurServer, handler.NewAURHandler(aurSvc))
			}

			if err := s.InitAll(); err != nil {
				return errors.WrapErr(err, "failed to initialize services")
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
	cliutil.SetVersion(&cmd)
	cliutil.AddNoColorFlag(&cmd)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.AddCommand(aurCmd())
	cmd.AddCommand(migrateCmd())
	cmd.AddCommand(kvCmd())
	cmd.AddCommand(repoCmd())
	cmd.AddCommand(version.Command())

	return &cmd
}
