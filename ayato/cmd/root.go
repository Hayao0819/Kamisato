package cmd

import (
	"fmt"
	"log"

	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/router"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/conf"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

func rootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use: "ayato",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config
			var err error
			cfg, err := conf.LoadAyatoConfig()
			if err != nil {
				return err
			}

			// Init
			r, err := repository.New(cfg)
			if err != nil {
				return err
			}
			s := service.New(r)
			h := handler.New(s, cfg)
			m := middleware.New(cfg)

			// Init gin
			engine := gin.Default()
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
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	return &cmd
}

func Execute() error {
	return rootCmd().Execute()
}
