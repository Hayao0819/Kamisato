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
	"github.com/Hayao0819/Kamisato/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

func RootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use: "ayato",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config
			var err error
			cfg, err := conf.LoadAyatoConfig(cmd.Flags())
			if err != nil {
				return err
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

			slog.Debug("Config loaded", "port", cfg.Port, "debug", cfg.Debug, "names", cfg.RepoNames, "maxsize", cfg.MaxSize, "dbtype", cfg.Store.DBType, "storagetype", cfg.Store.StorageType)

			// Init
			r, err := repository.New(cfg)
			if err != nil {
				return err
			}
			s := service.New(r)
			h := handler.New(s, cfg)
			m := middleware.New(cfg)

			// Init gin
			engine := gin.New()
			engine.Use(gin.Recovery())
			engine.Use(utils.GinLog())
			router.SetRoute(engine, h, m)

			// Init pacman repository
			// if err := r.Init(false, nil); err != nil {
			// 	return err
			// }
			if err := s.InitAll(); err != nil {
				return err
			}

			// Start server
			log.Printf("Listening on port %d", cfg.Port)
			if err := engine.Run(fmt.Sprintf(":%d", cfg.Port)); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug mode")
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	return &cmd
}
