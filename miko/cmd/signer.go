package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/ginutil"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/signer"
)

// signerCmd runs the dedicated signer tier: it holds the host signing key and
// signs the packages build workers POST to it, so those workers can run keyless.
func signerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "signer",
		Short: "Run the package signing service (holds the key so build workers stay keyless)",
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			cfg, err := conf.LoadMikoConfig(cmd.Flags(), configFile)
			if err != nil {
				return err
			}
			if cfg.Debug {
				cliutil.UseColorLog(slog.LevelDebug)
				gin.SetMode(gin.DebugMode)
			} else {
				cliutil.UseColorLog(slog.LevelInfo)
				gin.SetMode(gin.ReleaseMode)
			}

			hostSigner, err := buildHostSigner(cmd.Context(), cfg)
			if err != nil {
				return errors.WrapErr(err, "failed to set up host signing key")
			}
			if hostSigner == nil {
				return errors.NewErr("signer service needs a signing key: set signing.key_dir or data_dir")
			}

			verifier := serviceKeyVerifier(cfg)
			if !verifier.Enabled() && !cfg.AllowUnauthenticated {
				return errors.NewErr("signer service requires api_keys; set one or explicitly set allow_unauthenticated=true")
			}
			if !verifier.Enabled() {
				slog.Warn("signer service authentication explicitly disabled")
			}

			srv := ginutil.NewServer(fmt.Sprintf(":%d", cfg.Port), signer.Handler(hostSigner, verifier, cfg.MaxSize))
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			slog.Info("signer service listening", "port", cfg.Port)
			return ginutil.ServeHTTP(ctx, srv, nil)
		},
	}
}
