package cmd

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	httpserver "github.com/Hayao0819/Kamisato/internal/http/server"
	lumineconfig "github.com/Hayao0819/Kamisato/lumine/config"
	"github.com/Hayao0819/Kamisato/lumine/server"
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lumine",
		Short: "Lumine is a frontend for Ayato",
		Args:  cmdline.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			cfg, err := lumineconfig.LoadLumineConfig(cmd.Flags(), configFile)
			if err != nil {
				return err
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
		SilenceUsage: true,
	}
	cmd.Flags().String("addr", ":8080", "address to listen on")
	cmd.Flags().String("ayato-url", "", "Ayato URL to proxy /api and /repo to (env: LUMINE_AYATO_URL)")
	cmd.Flags().String("auth-mode", "cookie", "auth delivery mode: cookie (same-origin BFF proxy) or bearer (SPA calls ayato cross-origin with a token)")
	cmd.Flags().BoolP("debug", "d", false, "Enable debug mode")
	cmd.Flags().StringP("config", "c", "", "Config file")
	cmdline.SetVersion(cmd)
	cmdline.AddNoColorFlag(cmd)
	cmd.AddCommand(cmdline.VersionCommand())
	return cmd
}
