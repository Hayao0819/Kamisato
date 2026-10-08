package auditcmd

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayato/config"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func Cmd() *cobra.Command { return newCommand(repository.NewRawKV) }

func newCommand(open func(repository.KVSettings) (kv.Store, error)) *cobra.Command {
	var prune bool
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Report KV entries not created by ayato; --prune deletes them",
		Args:  cmdline.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (returnedErr error) {
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			cfg, err := config.LoadAyatoConfig(cmd.Flags(), configFile)
			if err != nil {
				return err
			}
			cmdline.Setup(slog.LevelInfo, cmdline.ColorEnabled(cmd))

			settings, err := config.RepositorySettings(cfg)
			if err != nil {
				return err
			}
			store, err := open(settings.KV)
			if err != nil {
				return errors.WrapErr(err, "failed to open kv store")
			}
			defer func() { returnedErr = errors.Join(returnedErr, store.Close()) }()
			auditor, ok := store.(kv.KeyAuditor)
			if !ok {
				return errors.New("the configured kv backend does not support key auditing")
			}
			return run(cmd.OutOrStdout(), auditor, prune)
		},
	}
	cmd.Flags().BoolVar(&prune, "prune", false, "delete the foreign keys (default: report only)")
	return cmd
}

func run(out io.Writer, auditor kv.KeyAuditor, prune bool) error {
	foreign, err := kv.AuditForeignKeys(auditor, prune)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "foreign keys: %d\n", len(foreign))
	for _, key := range foreign {
		fmt.Fprintf(out, "  %s\n", key)
	}
	if prune && len(foreign) > 0 {
		slog.Info("pruned foreign kv keys", "count", len(foreign))
	}
	return nil
}
