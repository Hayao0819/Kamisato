package cmd

import (
	"fmt"
	"log"

	"github.com/Hayao0819/Kamisato/ayato/router"
	"github.com/Hayao0819/Kamisato/conf"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

func rootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use: "ayato",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			config, err := conf.LoadAyatoConfig()
			if err != nil {
				return err
			}

			engine := gin.Default()

			router.SetRoute(engine)

			log.Printf("Listening on port %d", config.Port)
			if err := engine.Run(fmt.Sprintf(":%d", config.Port)); err != nil {
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
