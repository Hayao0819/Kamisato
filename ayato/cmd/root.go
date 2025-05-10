package cmd

import (
	"fmt"
	"log"

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
			var err error
			cfg, err := conf.LoadAyatoConfig()
			if err != nil {
				return err
			}

			r, err := repository.New(cfg)
			if err != nil {
				return err
			}
			s := service.NewService(r)

			if cfg == nil {
				return fmt.Errorf("config is nil")
			}

			engine := gin.Default()
			router.SetRoute(engine, cfg, s)
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
