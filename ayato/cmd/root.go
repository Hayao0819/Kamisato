package cmd

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	aurcmd "github.com/Hayao0819/Kamisato/ayato/cmd/aur"
	kvcmd "github.com/Hayao0819/Kamisato/ayato/cmd/kv"
	migratecmd "github.com/Hayao0819/Kamisato/ayato/cmd/migrate"
	repocmd "github.com/Hayao0819/Kamisato/ayato/cmd/repo"
	"github.com/Hayao0819/Kamisato/ayato/config"
	"github.com/Hayao0819/Kamisato/ayato/server"

	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	httpserver "github.com/Hayao0819/Kamisato/internal/http/server"
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ayato",
		Short: "Run the repository and build-gateway server",
		Args:  cmdline.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			cfg, err := config.LoadAyatoConfig(cmd.Flags(), configFile)
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
	cmdline.SetVersion(cmd)
	cmdline.AddNoColorFlag(cmd)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.AddCommand(aurcmd.Cmd(), migratecmd.Cmd(), kvcmd.Cmd(), repocmd.Cmd())
	cmd.AddCommand(cmdline.VersionCommand())

	return cmd
}
