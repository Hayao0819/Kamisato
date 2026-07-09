package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	sloggin "github.com/samber/slog-gin"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/auth/apikey"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/version"
	"github.com/Hayao0819/Kamisato/miko/handler"
	"github.com/Hayao0819/Kamisato/miko/router"
	"github.com/Hayao0819/Kamisato/miko/service"
	"github.com/Hayao0819/Kamisato/miko/signer"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

// buildSigner returns the worker's package signer per config: the local host key
// signing inline (default), or a remote signer that offloads to a dedicated signer
// tier so the worker holds no private key.
func buildSigner(ctx context.Context, cfg *conf.MikoConfig) (sign.Signer, error) {
	switch cfg.Signing.Mode {
	case "", "local":
		return buildHostSigner(ctx, cfg)
	case "remote":
		if cfg.Signing.Remote.URL == "" {
			return nil, errors.NewErr("signing.mode is remote but signing.remote.url is unset")
		}
		apiKey := cfg.Signing.Remote.APIKey
		if env := os.Getenv("MIKO_SIGNING_REMOTE_API_KEY"); env != "" {
			apiKey = env
		}
		slog.Info("remote signing enabled", "url", cfg.Signing.Remote.URL)
		return signer.NewRemoteSigner(cfg.Signing.Remote.URL, apiKey), nil
	default:
		return nil, errors.NewErrf("signing.mode: unknown value %q (want local or remote)", cfg.Signing.Mode)
	}
}

// buildHostSigner loads (or on first boot generates) the worker host signing key.
// It returns a nil Signer when no key dir is resolvable, leaving signing disabled.
func buildHostSigner(ctx context.Context, cfg *conf.MikoConfig) (sign.Signer, error) {
	dir := cfg.Signing.KeyDir
	if dir == "" && cfg.DataDir != "" {
		dir = filepath.Join(cfg.DataDir, "keys")
	}
	if dir == "" {
		slog.Warn("host signing disabled: set signing.key_dir or data_dir to enable")
		return nil, nil
	}
	name := cfg.Signing.Name
	if name == "" {
		name = "miko worker"
	}
	email := cfg.Signing.Email
	if email == "" {
		email = "miko@localhost"
	}
	// The passphrase comes only from the environment, never a config file.
	passphrase := os.Getenv("MIKO_SIGNING_PASSPHRASE")
	if passphrase == "" {
		slog.Warn("host signing key is stored unencrypted at rest; set MIKO_SIGNING_PASSPHRASE to encrypt it")
	}
	ks, err := sign.OpenOrCreate(dir, name, email, passphrase)
	if err != nil {
		return nil, err
	}
	slog.Info("host signing enabled", "key_dir", dir, //nolint:gosec // slog escapes structured values; dir is operator-provided config
		"master_fpr", fmt.Sprintf("%X", ks.MasterEntity().PrimaryKey.Fingerprint),
		"worker_fpr", fmt.Sprintf("%X", ks.WorkerEntity().PrimaryKey.Fingerprint))

	// Best-effort: register this worker's cert with ayato so its host-signed
	// packages verify. ayato accepts it only if it chains to a trusted master.
	if cfg.Ayato.URL != "" {
		if rerr := service.RegisterWorkerCert(ctx, cfg, ks); rerr != nil {
			slog.Warn("could not register worker key with ayato; signing still works, register it later", "err", rerr)
		}
	}
	return sign.NewHostKeySigner(ks), nil
}

func RootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use: "miko",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			if cfg.Debug {
				cliutil.Setup(slog.LevelDebug, cliutil.ColorEnabled(cmd))
				slog.Debug("Debug mode enabled")
				gin.SetMode(gin.DebugMode)
			} else {
				cliutil.Setup(slog.LevelInfo, cliutil.ColorEnabled(cmd))
				gin.SetMode(gin.ReleaseMode)
			}

			slog.Debug("Configuration loaded", "port", cfg.Port, "debug", cfg.Debug, "executor", cfg.Executor)

			pkgSigner, err := buildSigner(cmd.Context(), cfg)
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
			uploader := service.NewBlinkyUploader(cfg.Ayato.URL, cfg.Ayato.Username, cfg.Ayato.Password)

			s := service.New(cfg, pkgSigner, persister, uploader)
			h := handler.New(s, cfg)
			verifier := apikey.NewVerifier(cfg.APIKeys)

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			for i := 0; i < cfg.Concurrency; i++ {
				go s.Run(ctx)
			}
			slog.Info("Build workers launched", "concurrency", cfg.Concurrency)

			engine := gin.New()
			engine.Use(gin.Recovery())
			engine.Use(sloggin.NewWithConfig(slog.Default(), sloggin.Config{DefaultLevel: slog.LevelDebug, HandleGinDebug: true}))
			if err := router.SetRoute(engine, h, verifier); err != nil {
				return errors.WrapErr(err, "failed to set routing")
			}
			slog.Info("Routing initialized")

			// No WriteTimeout: it would kill healthy long-lived SSE log streams.
			// The per-flush write deadline in JobLogsHandler covers stuck writers.
			srv := &http.Server{
				Addr:              fmt.Sprintf(":%d", cfg.Port),
				Handler:           engine,
				ReadHeaderTimeout: 10 * time.Second,
				IdleTimeout:       120 * time.Second,
			}
			go func() {
				slog.Info("Waiting on port", "port", cfg.Port)
				if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					slog.Error("server error", "error", err)
					stop()
				}
			}()

			<-ctx.Done()
			slog.Info("Shutting down")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := srv.Shutdown(shutdownCtx); err != nil {
				return errors.WrapErr(err, "graceful shutdown failed")
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

	cmd.AddCommand(apikeyCmd())
	cmd.AddCommand(nvcheckCmd())
	cmd.AddCommand(signerCmd())
	cmd.AddCommand(version.Command())

	return &cmd
}
