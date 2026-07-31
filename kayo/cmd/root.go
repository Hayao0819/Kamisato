package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/ginutil"
	"github.com/Hayao0819/Kamisato/kayo/app"
	auditcmd "github.com/Hayao0819/Kamisato/kayo/cmd/audit"
	ayatocmd "github.com/Hayao0819/Kamisato/kayo/cmd/ayato"
	hookcmd "github.com/Hayao0819/Kamisato/kayo/cmd/hook"
	trustcmd "github.com/Hayao0819/Kamisato/kayo/cmd/trust"
	updatecmd "github.com/Hayao0819/Kamisato/kayo/cmd/update"
	verifycmd "github.com/Hayao0819/Kamisato/kayo/cmd/verify"
	kayoconfig "github.com/Hayao0819/Kamisato/kayo/config"
)

func RootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "kayo",
		Short: "Local aurweb-compatible overlay that intervenes in AUR-helper resolution",
		Args:  cliutil.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			cfg, err := kayoconfig.LoadKayoConfig(cmd.Flags(), configFile)
			if err != nil {
				return err
			}
			ginutil.Setup(cmd, cfg.Debug)
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return app.Run(ctx, cfg)
		},
	}
	cmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug mode")
	cmd.PersistentFlags().StringP("config", "c", "", "Config file")
	cmd.Flags().String("addr", "", "Listen address (host:port, default 127.0.0.1:10713)")
	cliutil.SetVersion(&cmd)
	cliutil.AddNoColorFlag(&cmd)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.AddCommand(auditcmd.Cmd(), trustcmd.Cmd(), updatecmd.Cmd(), verifycmd.Cmd(), hookcmd.Cmd(), ayatocmd.Cmd(), cliutil.VersionCommand())
	return &cmd
}
