package cmd

import (
	"fmt"
	"log"
	"log/slog"

	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/router"
	"github.com/Hayao0819/Kamisato/ayato/service"
	utils "github.com/Hayao0819/Kamisato/internal"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

func RootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use: "ayato",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config file flag
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}

			// Load config
			cfg, err := conf.LoadAyatoConfig(cmd.Flags(), configFile)
			if err != nil {
				return err
			}

			if configFile != "" {
				slog.Info("Loading config from file", "path", configFile)
			}

			// Init logger
			if cfg.Debug {
				// println("debug mode")
				utils.UseColorLog(slog.LevelDebug)
				slog.Debug("Debug mode enabled")
				gin.SetMode(gin.DebugMode)
			} else {
				utils.UseColorLog(slog.LevelInfo)
				gin.SetMode(gin.ReleaseMode)
			}

			slog.Debug("Config loaded", "port", cfg.Port, "debug", cfg.Debug, "repos", cfg.Repos, "maxsize", cfg.MaxSize, "dbtype", cfg.Store.DBType, "storagetype", cfg.Store.StorageType)

			// Init
			r, err := repository.New(cfg)
			if err != nil {
				return errors.Wrap(err, "failed to initialize repository")
			}
			s := service.New(r)
			h := handler.New(s, cfg)
			m := middleware.New(cfg)

			// Init gin
			engine := gin.New()
			engine.Use(gin.Recovery())
			engine.Use(utils.GinLog())
			router.SetRoute(engine, h, m)
			slog.Info("Routes initialized successfully")

			// Initialize package repository
			if err := s.InitAll(); err != nil {
				return errors.Wrap(err, "failed to initialize services")
			}
			slog.Info("All services initialized successfully")

			// Start server
			log.Printf("Listening on port %d", cfg.Port)
			if err := engine.Run(fmt.Sprintf(":%d", cfg.Port)); err != nil {
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
