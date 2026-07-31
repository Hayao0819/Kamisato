package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayato/app"
	ayatoconfig "github.com/Hayao0819/Kamisato/ayato/config"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
)

func auditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Report KV entries not created by ayato; --prune deletes them",
		Args:  cliutil.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			cfg, err := ayatoconfig.LoadAyatoConfig(cmd.Flags(), configFile)
			if err != nil {
				return err
			}
			cliutil.Setup(slog.LevelInfo, cliutil.ColorEnabled(cmd))

			prune, _ := cmd.Flags().GetBool("prune")
			foreign, err := app.Audit(cfg, prune)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "foreign keys: %d\n", len(foreign))
			for _, k := range foreign {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", k)
			}

			if prune && len(foreign) > 0 {
				slog.Info("pruned foreign kv keys", "count", len(foreign))
			}
			return nil
		},
	}
	cmd.Flags().Bool("prune", false, "delete the foreign keys (default: report only)")
	return cmd
}
