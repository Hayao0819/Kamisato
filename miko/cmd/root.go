package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/ginutil"
	"github.com/Hayao0819/Kamisato/internal/version"
	apikeycmd "github.com/Hayao0819/Kamisato/miko/cmd/apikey"
	nvcheckcmd "github.com/Hayao0819/Kamisato/miko/cmd/nvcheck"
	"github.com/Hayao0819/Kamisato/miko/cmd/shared"
	signercmd "github.com/Hayao0819/Kamisato/miko/cmd/signer"
	"github.com/Hayao0819/Kamisato/miko/handler"
	"github.com/Hayao0819/Kamisato/miko/router"
	"github.com/Hayao0819/Kamisato/miko/service"
)

func RootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:  "miko",
		RunE: run,
	}
	cmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug mode")
	cmd.PersistentFlags().StringP("config", "c", "", "Config file")
	cliutil.SetVersion(&cmd)
	cliutil.AddNoColorFlag(&cmd)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	cmd.AddCommand(apikeycmd.Cmd())
	cmd.AddCommand(nvcheckcmd.Cmd())
	cmd.AddCommand(signercmd.Cmd())
	cmd.AddCommand(version.Command())

	return &cmd
}

func run(cmd *cobra.Command, _ []string) error {
	configFile, err := cmd.Flags().GetString("config")
	if err != nil {
		return err
	}

	cfg, err := conf.LoadMikoConfig(cmd.Flags(), configFile)
	if err != nil {
		return err
	}

	if configFile != "" {
		slog.Info("Loaded from config file", "path", configFile)
	}

	ginutil.Setup(cmd, cfg.Debug)

	slog.Debug("Configuration loaded", "port", cfg.Port, "debug", cfg.Debug, "executor", cfg.Executor)

	pkgSigner, err := shared.BuildSigner(cmd.Context(), cfg)
	if err != nil {
		return errors.WrapErr(err, "failed to set up package signing")
	}

	var persister service.Persister
	if cfg.DataDir != "" {
		p, perr := service.NewFilePersister(cfg.DataDir)
		if perr != nil {
			slog.Error("job persistence disabled", "error", perr)
		} else {
			persister = p
		}
	}
	uploader, err := service.NewAyatoUploader(cfg.Ayato.URL, cfg.Ayato.APIKey)
	if err != nil {
		return errors.WrapErr(err, "failed to configure Ayato publisher")
	}
	serviceOptions, err := shared.ServiceDependencies(cfg)
	if err != nil {
		return err
	}
	serviceOptions = append(
		serviceOptions,
		service.WithSigner(pkgSigner),
		service.WithPersister(persister),
		service.WithUploader(uploader),
	)

	s := service.New(cfg, serviceOptions...)
	h := handler.New(s, cfg)
	verifier := shared.ServiceKeyVerifier(cfg)
	if !verifier.Enabled() && !cfg.AllowUnauthenticated {
		return errors.NewErr("no api_keys configured; set one or explicitly set allow_unauthenticated=true")
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serviceDone := make(chan struct{})
	go func() {
		defer close(serviceDone)
		s.Run(ctx)
	}()
	slog.Info("Build workers launched", "concurrency", cfg.Concurrency)

	engine := ginutil.NewEngine()
	if err := router.SetRoute(engine, h, verifier); err != nil {
		return errors.WrapErr(err, "failed to set routing")
	}
	slog.Info("Routing initialized")

	srv := ginutil.NewServer(fmt.Sprintf(":%d", cfg.Port), engine)
	slog.Info("Waiting on port", "port", cfg.Port)
	serveErr := ginutil.ServeHTTP(ctx, srv, nil)
	// A serve failure returns without ctx being cancelled; stop the
	// workers explicitly or the wait below would never end.
	stop()

	var workerErr error
	select {
	case <-serviceDone:
	case <-time.After(15 * time.Second):
		workerErr = errors.NewErr("build workers did not stop before shutdown deadline")
	}
	return errors.Join(serveErr, workerErr)
}
