package migratecmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayato/config"
	"github.com/Hayao0819/Kamisato/ayato/migrate"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// Cmd runs a migration as a one-shot job. Run it separately (e.g. a Cloud Run
// Job), never inside the serving service, which throttles CPU and caps request time.
func Cmd() *cobra.Command {
	var status bool
	var phase string
	var options migrate.RunOptions
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run data-layout migrations as a one-shot job",
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

			// K_SERVICE/K_REVISION mark a Cloud Run service; a Job has neither.
			if config.UnderCloudRun() {
				slog.Warn("ayato migrate is running inside a Cloud Run service, not a Job; run it as a Cloud Run Job — a service throttles CPU outside requests and caps requests at 60 minutes")
			}

			if !status && phase != string(migrate.PhaseExpand) && phase != string(migrate.PhaseContract) {
				return errors.New("--phase must be expand or contract")
			}
			settings, err := config.RepositorySettings(cfg)
			if err != nil {
				return err
			}
			kvStore, blobStore, err := repository.NewMigrationStores(settings)
			if err != nil {
				return errors.WrapErr(err, "failed to open stores")
			}
			defer func() { returnedErr = errors.Join(returnedErr, kvStore.Close()) }()
			if status {
				result, err := migrate.Statuses(kvStore, migrate.Registered())
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "layout_version: %d\n", result.Layout)
				for _, m := range result.Migrations {
					fmt.Fprintf(cmd.OutOrStdout(), "  %d %-16s expanded=%t contracted=%t\n", m.Version, m.Name, m.Expanded, m.Contracted)
				}
				return nil
			}

			options.Phase = migrate.Phase(phase)
			result, err := migrate.Run(cmd.Context(), &migrate.Stores{KV: kvStore, Blob: blobStore}, migrate.Registered(), options)
			slog.Info("migration run", "phase", result.Phase, "applied", result.Applied, "skipped", result.Skipped, "dryRun", options.DryRun)
			return err
		},
	}
	cmd.Flags().StringVar(&phase, "phase", "", "migration phase: expand (additive) or contract (cleanup)")
	cmd.Flags().IntVar(&options.To, "to", 0, "run up to and including this version (0 = all)")
	cmd.Flags().BoolVar(&options.DryRun, "dry-run", false, "log the plan without mutating anything")
	cmd.Flags().BoolVar(&status, "status", false, "print the layout version and per-migration state")
	return cmd
}
