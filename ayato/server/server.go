package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/bugreport"
	ayatoconfig "github.com/Hayao0819/Kamisato/ayato/config"
	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/handler/recaptcha"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/ayato/migrate"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/router"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/errors"
	httpserver "github.com/Hayao0819/Kamisato/internal/http/server"
)

func Run(ctx context.Context, cfg *ayatoconfig.AyatoConfig) (runErr error) {
	slog.Debug("Configuration loaded",
		"port", cfg.Port,
		"debug", cfg.Debug,
		"repos", cfg.Repos,
		"maxsize", cfg.MaxSize,
		"dbtype", cfg.Store.DBType,
		"storagetype", cfg.Store.StorageType,
	)
	repoSettings, err := ayatoconfig.RepositorySettings(cfg)
	if err != nil {
		return err
	}
	pkgNameRepo, pkgBinaryRepo, authRepo, kvStore, err := repository.New(repoSettings)
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
	settings, err := serviceSettings(cfg)
	if err != nil {
		return err
	}
	outboundClient := &http.Client{Timeout: 30 * time.Second}
	appService := service.New(pkgNameRepo, pkgBinaryRepo, authRepo, signerRepo, settings,
		service.WithOutboundHTTPClient(outboundClient))
	verificationClient := &http.Client{Timeout: 10 * time.Second, Transport: outboundClient.Transport}
	reporter, err := bugreport.New(bugReportSettings(cfg.BugReport), verificationClient)
	if err != nil {
		slog.Error("bug reporting disabled: invalid config", "error", err)
	}
	verifier := recaptcha.NewWithHTTPClient(cfg.Recaptcha.Provider, cfg.Recaptcha.Secret, verificationClient)
	appHandler := handler.New(appService, HandlerSettings(cfg), outboundClient, reporter, verifier).WithLogTokens(logTokenRepo)
	appMiddleware := middleware.New(middlewareSettings(cfg)).WithLogTokens(logTokenRepo).WithRateLimiter(kvStore)

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

	ci, err := auth.NewCIAuthorizerWithHTTPClient(ctx, ciSettings(cfg.Auth.CI), verificationClient)
	if err != nil {
		return errors.WrapErr(err, "failed to init CI auth")
	}
	appMiddleware.WithCIAuth(ci)
	if cfg.Auth.AllowLegacySignerBasic {
		slog.Warn("legacy Basic authentication is enabled only for signer registration; deploy Ayato before Miko, then disable auth.allow_legacy_signer_basic after the rollback window")
	}

	state := &httpserver.Readiness{}
	engine, err := buildRouter(cfg, appHandler, appMiddleware, kvStore, state)
	if err != nil {
		return err
	}
	if err := appService.InitAll(); err != nil {
		return errors.WrapErr(err, "failed to initialize services")
	}
	slog.Info("All services initialized")

	server := httpserver.NewServer(fmt.Sprintf(":%d", cfg.Port), engine)
	slog.Info("Waiting on port", "port", cfg.Port)
	return httpserver.ServeHTTP(ctx, server, state)
}

func buildRouter(
	cfg *ayatoconfig.AyatoConfig,
	appHandler *handler.Set,
	appMiddleware *middleware.Middleware,
	kvStore kv.Store,
	state *httpserver.Readiness,
) (*gin.Engine, error) {
	engine := httpserver.NewEngine()
	engine.Use(
		appMiddleware.SecurityHeaders(),
		appMiddleware.RejectMutationsWhenNotReady(state),
	)

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
