package cmd

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/router"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/conf"
	utils "github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

// RootCmd returns the root command for Ayato CLI.
// Returns the root command for Ayato CLI.
func RootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use: "ayato",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get config file flag
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}

			// Load configuration
			cfg, err := conf.LoadAyatoConfig(cmd.Flags(), configFile)
			if err != nil {
				return err
			}

			if configFile != "" {
				slog.Info("Loaded from config file", "path", configFile)
			}

			// Initialize logger
			if cfg.Debug {
				utils.UseColorLog(slog.LevelDebug)
				slog.Debug("Debug mode enabled")
				gin.SetMode(gin.DebugMode)

				// Set debug logger for gin
				// https://github.com/samber/slog-gin/issues/54
				//
				// gin.DebugPrintRouteFunc = func(httpMethod string, absolutePath string, handlerName string, nuHandlers int) {
				// 	slog.Debug("Route registered", "method", httpMethod, "path", absolutePath, "handler", handlerName, "num_handlers", nuHandlers)
				// }
				// gin.DebugPrintFunc = func(format string, values ...any) {
				// 	str := strings.Trim(fmt.Sprintf("[GIN-debug] %s", fmt.Sprintf(format, values...)), "\n")
				// 	slog.Debug(str)
				// }
			} else {
				utils.UseColorLog(slog.LevelInfo)
				gin.SetMode(gin.ReleaseMode)
			}

			slog.Debug("Configuration loaded", "port", cfg.Port, "debug", cfg.Debug, "repos", cfg.Repos, "maxsize", cfg.MaxSize, "dbtype", cfg.Store.DBType, "storagetype", cfg.Store.StorageType)

			// Initialize repository, service, handler
			r, err := repository.New(cfg)
			if err != nil {
				return utils.WrapErr(err, "failed to initialize repository")
			}
			s := service.New(r, cfg)
			h := handler.New(s, cfg)
			m := middleware.New(cfg)

			// Initialize gin
			engine := gin.New()
			engine.Use(gin.Recovery())
			engine.Use(utils.GinLog())
			if err := router.SetRoute(engine, h, m); err != nil {
				return utils.WrapErr(err, "failed to set routing")
			}
			slog.Info("Routing initialized")

			// Initialize services
			if err := s.InitAll(); err != nil {
				return utils.WrapErr(err, "failed to initialize services")
			}
			slog.Info("All services initialized")

			// Start server
			slog.Info("Waiting on port", "port", cfg.Port)
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
