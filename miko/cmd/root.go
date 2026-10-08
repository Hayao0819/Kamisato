package cmd

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	httpserver "github.com/Hayao0819/Kamisato/internal/http/server"

	apikeycmd "github.com/Hayao0819/Kamisato/miko/cmd/apikey"
	nvcheckcmd "github.com/Hayao0819/Kamisato/miko/cmd/nvcheck"
	signercmd "github.com/Hayao0819/Kamisato/miko/cmd/signer"
	"github.com/Hayao0819/Kamisato/miko/config"
	"github.com/Hayao0819/Kamisato/miko/server"
)

func RootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "miko",
		Short: "Run the package-build worker server",
		Args:  cmdline.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			cfg, err := config.LoadMikoConfig(cmd.Flags(), configFile)
			if err != nil {
				return err
			}
			if configFile != "" {
				slog.Info("Loaded from config file", "path", configFile)
			}
			level := slog.LevelInfo
			if cfg.Debug {
				level = slog.LevelDebug
			}
			cmdline.Setup(level, cmdline.ColorEnabled(cmd))
			httpserver.SetMode(cfg.Debug)
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return server.Run(ctx, cfg)
		},
	}
	cmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug mode")
	cmd.PersistentFlags().StringP("config", "c", "", "Config file")
	cmdline.SetVersion(&cmd)
	cmdline.AddNoColorFlag(&cmd)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.AddCommand(apikeycmd.Cmd(), nvcheckcmd.Cmd(), signercmd.Cmd(), cmdline.VersionCommand())
	return &cmd
}
