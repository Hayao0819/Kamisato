package cmd

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	httpserver "github.com/Hayao0819/Kamisato/internal/http/server"
	auditcmd "github.com/Hayao0819/Kamisato/kayo/cmd/audit"
	ayatocmd "github.com/Hayao0819/Kamisato/kayo/cmd/ayato"
	hookcmd "github.com/Hayao0819/Kamisato/kayo/cmd/hook"
	trustcmd "github.com/Hayao0819/Kamisato/kayo/cmd/trust"
	updatecmd "github.com/Hayao0819/Kamisato/kayo/cmd/update"
	verifycmd "github.com/Hayao0819/Kamisato/kayo/cmd/verify"
	kayoconfig "github.com/Hayao0819/Kamisato/kayo/config"
	"github.com/Hayao0819/Kamisato/kayo/server"
)

func RootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "kayo",
		Short: "Local aurweb-compatible overlay that intervenes in AUR-helper resolution",
		Args:  cmdline.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			cfg, err := kayoconfig.LoadKayoConfig(cmd.Flags(), configFile)
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
	}
	cmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug mode")
	cmd.PersistentFlags().StringP("config", "c", "", "Config file")
	cmd.Flags().String("addr", "", "Listen address (host:port, default 127.0.0.1:10713)")
	cmdline.SetVersion(&cmd)
	cmdline.AddNoColorFlag(&cmd)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.AddCommand(auditcmd.Cmd(), trustcmd.Cmd(), updatecmd.Cmd(), verifycmd.Cmd(), hookcmd.Cmd(), ayatocmd.Cmd(), cmdline.VersionCommand())
	return &cmd
}
