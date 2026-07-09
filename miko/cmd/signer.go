package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/auth/apikey"
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

			verifier := apikey.NewVerifier(cfg.APIKeys)
			if !verifier.Enabled() {
				slog.Warn("signer service has no api_keys configured; it trusts the closed network only")
			}

			srv := &http.Server{
				Addr:              fmt.Sprintf(":%d", cfg.Port),
				Handler:           signer.Handler(hostSigner, verifier),
				ReadHeaderTimeout: 10 * time.Second,
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			go func() {
				slog.Info("signer service listening", "port", cfg.Port)
				if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					slog.Error("signer server error", "error", err)
					stop()
				}
			}()

			<-ctx.Done()
			slog.Info("Shutting down signer service")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			return srv.Shutdown(shutdownCtx)
		},
	}
}
