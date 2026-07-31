package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/ginutil"
	"github.com/Hayao0819/Kamisato/lumine/app"
	lumineconfig "github.com/Hayao0819/Kamisato/lumine/config"
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lumine",
		Short: "Lumine is a frontend for Ayato",
		Args:  cliutil.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			cfg, err := lumineconfig.LoadLumineConfig(cmd.Flags(), configFile)
			if err != nil {
				return err
			}
			ginutil.Setup(cmd, cfg.Debug)
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return app.Run(ctx, cfg)
		},
		SilenceUsage: true,
	}
	cmd.Flags().String("addr", ":8080", "address to listen on")
	cmd.Flags().String("ayato-url", "", "Ayato URL to proxy /api and /repo to (env: LUMINE_AYATO_URL)")
	cmd.Flags().String("auth-mode", "cookie", "auth delivery mode: cookie (same-origin BFF proxy) or bearer (SPA calls ayato cross-origin with a token)")
	cmd.Flags().BoolP("debug", "d", false, "Enable debug mode")
	cmd.Flags().StringP("config", "c", "", "Config file")
	cliutil.SetVersion(cmd)
	cliutil.AddNoColorFlag(cmd)
	cmd.AddCommand(cliutil.VersionCommand())
	return cmd
}
