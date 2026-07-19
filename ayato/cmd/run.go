package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	sloggin "github.com/samber/slog-gin"
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/lifecycle"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/ayato/migrate"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/router"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func run(cmd *cobra.Command, _ []string) error {
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
	setupRuntime(cmd, cfg)

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runServer(ctx, cfg)
}

func setupRuntime(cmd *cobra.Command, cfg *conf.AyatoConfig) {
	if cfg.Debug {
		cliutil.Setup(slog.LevelDebug, cliutil.ColorEnabled(cmd))
		slog.Debug("Debug mode enabled")
		gin.SetMode(gin.DebugMode)
	} else {
		cliutil.Setup(slog.LevelInfo, cliutil.ColorEnabled(cmd))
		gin.SetMode(gin.ReleaseMode)
	}
	slog.Debug("Configuration loaded",
		"port", cfg.Port,
		"debug", cfg.Debug,
		"repos", cfg.Repos,
		"maxsize", cfg.MaxSize,
		"dbtype", cfg.Store.DBType,
		"storagetype", cfg.Store.StorageType,
	)
}

func runServer(ctx context.Context, cfg *conf.AyatoConfig) (runErr error) {
	pkgNameRepo, pkgBinaryRepo, authRepo, kvStore, err := repository.New(cfg)
	if err != nil {
		return errors.WrapErr(err, "failed to initialize repository")
	}
	defer func() {
		runErr = errors.Join(runErr, errors.WrapErr(kvStore.Close(), "failed to close key-value store"))
	}()

	version, inRange, err := migrate.Guard(kvStore, migrate.SupportedMin, migrate.SupportedMax)
	if err != nil {
		return errors.WrapErr(err, "failed to read repository layout version")
	}
	if !inRange {
		return errors.NewErrf(
			"repository layout version %d is outside this binary's supported range [%d, %d]",
			version,
			migrate.SupportedMin,
			migrate.SupportedMax,
		)
	}

	signerRepo := repository.NewSignerRepository(kvStore)
	denylistRepo := repository.NewDenylistRepository(kvStore)
	replayGuard := repository.NewReplayGuard(kvStore)
	logTokenRepo := repository.NewLogTokenRepository(kvStore)
	deviceRepo := repository.NewDeviceRepository(kvStore)
	appService := service.New(pkgNameRepo, pkgBinaryRepo, authRepo, signerRepo, cfg)
	appHandler := handler.New(appService, cfg).WithLogTokens(logTokenRepo)
	appMiddleware := middleware.New(cfg).WithLogTokens(logTokenRepo).WithRateLimiter(kvStore)

	if err := appService.SeedBootstrapAdmin(cfg.Auth.BootstrapAdminGitHubID); err != nil {
		return errors.WrapErr(err, "failed to seed bootstrap admin")
	}
	if len(cfg.Auth.SessionSecret) > 0 {
		if _, ok := kvStore.(kv.Adder); !ok {
			return errors.NewErr("authentication requires atomic refresh-token consumption, but the configured KV store does not implement it; use SQL or BadgerDB")
		}
		signer, signerErr := auth.NewSigner(cfg.Auth.SessionSecret)
		if signerErr != nil {
			return errors.WrapErr(signerErr, "failed to build session signer")
		}
		appHandler.WithAuth(signer).WithReplayGuard(replayGuard).WithDeviceStore(deviceRepo)
		appService.WithDenylist(denylistRepo)
		appMiddleware.WithAuth(appService, signer).WithDenylist(denylistRepo)
	} else {
		slog.Warn("authentication is not configured; mutating and admin routes will fail closed (503) until auth.session_secret and auth.github are set")
	}

	ci, err := auth.NewCIAuthorizer(ctx, cfg.Auth.CI)
	if err != nil {
		return errors.WrapErr(err, "failed to init CI auth")
	}
	appMiddleware.WithCIAuth(ci)
	if cfg.Auth.AllowLegacySignerBasic {
		slog.Warn("legacy Basic authentication is enabled only for signer registration; deploy Ayato before Miko, then disable auth.allow_legacy_signer_basic after the rollback window")
	}

	state := &lifecycle.State{}
	engine, err := buildRouter(cfg, appHandler, appMiddleware, kvStore, state)
	if err != nil {
		return err
	}
	if err := appService.InitAll(); err != nil {
		return errors.WrapErr(err, "failed to initialize services")
	}
	slog.Info("All services initialized")

	// No ReadTimeout or WriteTimeout: large bounded uploads, repository downloads,
	// and proxied miko SSE streams can legitimately be long-lived.
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	slog.Info("Waiting on port", "port", cfg.Port)
	return lifecycle.Serve(ctx, server, state)
}

func buildRouter(
	cfg *conf.AyatoConfig,
	appHandler *handler.Set,
	appMiddleware *middleware.Middleware,
	kvStore kv.Store,
	state *lifecycle.State,
) (*gin.Engine, error) {
	engine := gin.New()
	engine.Use(
		gin.Recovery(),
		sloggin.NewWithConfig(slog.Default(), sloggin.Config{DefaultLevel: slog.LevelDebug, HandleGinDebug: true}),
		appMiddleware.SecurityHeaders(),
		appMiddleware.RejectMutationsWhenNotReady(state),
	)

	if err := engine.SetTrustedProxies(nil); err != nil {
		return nil, errors.WrapErr(err, "failed to reset trusted proxies")
	}
	if len(cfg.Auth.TrustedProxies) > 0 {
		if err := engine.SetTrustedProxies(cfg.Auth.TrustedProxies); err != nil {
			return nil, errors.WrapErr(err, "failed to set trusted proxies")
		}
	}

	if err := router.SetRoute(engine, appHandler, appMiddleware, router.WithReadiness(state)); err != nil {
		return nil, errors.WrapErr(err, "failed to set routing")
	}
	if cfg.AUR.Enabled {
		aurServer, aurService, err := buildAUR(cfg, kvStore)
		if err != nil {
			return nil, errors.WrapErr(err, "failed to initialize AUR module")
		}
		router.SetAUR(engine, appMiddleware, aurServer, handler.NewAURHandler(aurService))
	}
	slog.Info("Routing initialized")
	return engine, nil
}
