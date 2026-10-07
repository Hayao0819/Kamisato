package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"
	httpserver "github.com/Hayao0819/Kamisato/internal/http/server"
	mikoconfig "github.com/Hayao0819/Kamisato/miko/config"
	"github.com/Hayao0819/Kamisato/miko/handler"
	"github.com/Hayao0819/Kamisato/miko/router"
	"github.com/Hayao0819/Kamisato/miko/service"
)

func Run(ctx context.Context, cfg *mikoconfig.MikoConfig) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	slog.Debug("Configuration loaded", "port", cfg.Port, "debug", cfg.Debug, "executor", cfg.Executor)

	pkgSigner, err := BuildSigner(ctx, cfg)
	if err != nil {
		return errors.WrapErr(err, "failed to set up package signing")
	}

	var persister service.Persister
	if cfg.DataDir != "" {
		configured, persistErr := service.NewFilePersister(cfg.DataDir)
		if persistErr != nil {
			slog.Error("job persistence disabled", "error", persistErr)
		} else {
			persister = configured
		}
	}
	uploader, err := service.NewAyatoUploader(cfg.Ayato.URL, cfg.Ayato.APIKey)
	if err != nil {
		return errors.WrapErr(err, "failed to configure Ayato publisher")
	}
	serviceOptions, err := ServiceDependencies(cfg)
	if err != nil {
		return err
	}
	serviceOptions = append(
		serviceOptions,
		service.WithSigner(pkgSigner),
		service.WithPersister(persister),
		service.WithUploader(uploader),
	)

	serviceInstance := service.New(ServiceSettings(cfg), serviceOptions...)
	handlerInstance := handler.New(serviceInstance, handler.Settings{MaxLogReaders: cfg.MaxLogReaders})
	verifier := ServiceKeyVerifier(cfg)
	if !verifier.Enabled() && !cfg.AllowUnauthenticated {
		return errors.NewErr("no api_keys configured; set one or explicitly set allow_unauthenticated=true")
	}

	serviceDone := make(chan struct{})
	go func() {
		defer close(serviceDone)
		serviceInstance.Run(ctx)
	}()
	slog.Info("Build workers launched", "concurrency", cfg.Concurrency)

	engine := httpserver.NewEngine()
	if err := router.SetRoute(engine, handlerInstance, verifier); err != nil {
		return errors.WrapErr(err, "failed to set routing")
	}
	slog.Info("Routing initialized")

	server := httpserver.NewServer(fmt.Sprintf(":%d", cfg.Port), engine)
	slog.Info("Waiting on port", "port", cfg.Port)
	serveErr := httpserver.ServeHTTP(ctx, server, nil)
	cancel()

	var workerErr error
	select {
	case <-serviceDone:
	case <-time.After(15 * time.Second):
		workerErr = errors.NewErr("build workers did not stop before shutdown deadline")
	}
	return errors.Join(serveErr, workerErr)
}
