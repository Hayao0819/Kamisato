package cmd

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayato/app"
	ayatoconfig "github.com/Hayao0819/Kamisato/ayato/config"
	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	httpserver "github.com/Hayao0819/Kamisato/internal/http/server"
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "ayato",
		Args: cmdline.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			cfg, err := ayatoconfig.LoadAyatoConfig(cmd.Flags(), configFile)
			if err != nil {
				return err
			}
			if configFile != "" {
				slog.Info("Loaded from config file", "path", configFile)
			}
			httpserver.Setup(cmd, cfg.Debug)
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return app.Run(ctx, cfg)
		},
	}
	cmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug mode")
	cmd.PersistentFlags().StringP("config", "c", "", "Config file")
	cmdline.SetVersion(cmd)
	cmdline.AddNoColorFlag(cmd)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.AddCommand(aurCmd())
	cmd.AddCommand(migrateCmd())
	cmd.AddCommand(kvCmd())
	cmd.AddCommand(repoCmd())
	cmd.AddCommand(cmdline.VersionCommand())

	return cmd
}
