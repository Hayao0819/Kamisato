package signercmd

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	httpserver "github.com/Hayao0819/Kamisato/internal/http/server"
	"github.com/Hayao0819/Kamisato/miko/config"
	"github.com/Hayao0819/Kamisato/miko/server"
)

// Cmd runs the dedicated signer tier: it holds the host signing key and
// signs the packages build workers POST to it, so those workers can run keyless.
func Cmd() *cobra.Command {
	return &cobra.Command{
		Use:   "signer",
		Short: "Run the package signing service (holds the key so build workers stay keyless)",
		Args:  cmdline.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			cfg, err := config.LoadMikoConfig(cmd.Flags(), configFile)
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
			return server.RunSigner(ctx, cfg)
		},
	}
}
