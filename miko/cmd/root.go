package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Hayao0819/Kamisato/internal/apikey"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/internal/weblog"
	"github.com/Hayao0819/Kamisato/miko/handler"
	"github.com/Hayao0819/Kamisato/miko/router"
	"github.com/Hayao0819/Kamisato/miko/service"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

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
	slog.Info("host signing enabled", "key_dir", dir,
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
				utils.UseColorLog(slog.LevelDebug)
				slog.Debug("Debug mode enabled")
				gin.SetMode(gin.DebugMode)
			} else {
				utils.UseColorLog(slog.LevelInfo)
				gin.SetMode(gin.ReleaseMode)
			}

			slog.Debug("Configuration loaded", "port", cfg.Port, "debug", cfg.Debug, "executor", cfg.Executor)

			signer, err := buildHostSigner(cmd.Context(), cfg)
			if err != nil {
				return utils.WrapErr(err, "failed to set up host signing key")
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

			s := service.New(cfg, signer, persister, uploader)
			h := handler.New(s, cfg)
			verifier := apikey.NewVerifier(cfg.APIKeys)

			// Cancel the workers (and in-flight builds) on SIGINT/SIGTERM.
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			for i := 0; i < cfg.Concurrency; i++ {
				go s.Run(ctx)
			}
			slog.Info("Build workers launched", "concurrency", cfg.Concurrency)

			engine := gin.New()
			engine.Use(gin.Recovery())
			engine.Use(weblog.GinLog())
			if err := router.SetRoute(engine, h, verifier); err != nil {
				return utils.WrapErr(err, "failed to set routing")
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
				return utils.WrapErr(err, "graceful shutdown failed")
			}
			return nil
		},
	}
	cmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug mode")
	cmd.PersistentFlags().StringP("config", "c", "", "Config file")
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	cmd.AddCommand(apikeyCmd())

	return &cmd
}
