package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func kvCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "kv", Short: "Key-value store maintenance"}
	cmd.AddCommand(kvAuditCmd())
	return cmd
}

// kvAuditCmd reports (and with --prune deletes) KV entries ayato did not create,
// such as data injected through the provider's console. Run it as a one-shot job.
func kvAuditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Report KV entries not created by ayato; --prune deletes them",
		RunE: func(cmd *cobra.Command, _ []string) error {
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			cfg, err := conf.LoadAyatoConfig(cmd.Flags(), configFile)
			if err != nil {
				return err
			}
			cliutil.Setup(slog.LevelInfo, cliutil.ColorEnabled(cmd))

			store, err := repository.NewRawKV(cfg)
			if err != nil {
				return errors.WrapErr(err, "failed to open kv store")
			}
			defer func() { _ = store.Close() }()

			auditor, ok := store.(kv.KeyAuditor)
			if !ok {
				return errors.New("the configured kv backend does not support key auditing")
			}
			foreign, err := auditor.ForeignKeys()
			if err != nil {
				return err
			}
			fmt.Printf("foreign keys: %d\n", len(foreign))
			for _, k := range foreign {
				fmt.Printf("  %s\n", k)
			}

			prune, _ := cmd.Flags().GetBool("prune")
			if !prune || len(foreign) == 0 {
				return nil
			}
			if err := auditor.DeleteRawKeys(foreign); err != nil {
				return err
			}
			slog.Info("pruned foreign kv keys", "count", len(foreign))
			return nil
		},
	}
	cmd.Flags().Bool("prune", false, "delete the foreign keys (default: report only)")
	return cmd
}
