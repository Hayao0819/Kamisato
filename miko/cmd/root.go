package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Hayao0819/Kamisato/internal/apikey"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/miko/handler"
	"github.com/Hayao0819/Kamisato/miko/router"
	"github.com/Hayao0819/Kamisato/miko/service"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

// RootCmd returns the root command for the miko build server.
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

			if cfg.Executor == "" {
				cfg.Executor = "container"
			}
			if cfg.Port == 0 {
				cfg.Port = 8081
			}
			if cfg.Concurrency < 1 {
				cfg.Concurrency = 1
			}
			if cfg.MaxRetries < 0 {
				cfg.MaxRetries = 0
			}
			if cfg.RetryBackoff == 0 {
				cfg.RetryBackoff = 5
			}
			if cfg.MaxLogBytes == 0 {
				cfg.MaxLogBytes = 16 << 20
			}
			if cfg.MaxLogReaders == 0 {
				cfg.MaxLogReaders = 8
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

			s := service.New(cfg)
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
			engine.Use(utils.GinLog())
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
