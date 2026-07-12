package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayato/migrate"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// migrateCmd runs a migration as a one-shot job. Run it separately (e.g. a Cloud Run
// Job), never inside the serving service, which throttles CPU and caps request time.
func migrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run data-layout migrations as a one-shot job",
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

			// K_SERVICE/K_REVISION mark a Cloud Run service; a Job has neither.
			if conf.UnderCloudRun() {
				slog.Warn("ayato migrate is running inside a Cloud Run service, not a Job; run it as a Cloud Run Job — a service throttles CPU outside requests and caps requests at 60 minutes")
			}

			kvStore, blobStore, err := repository.NewMigrationStores(cfg)
			if err != nil {
				return errors.WrapErr(err, "failed to open stores")
			}
			defer func() { _ = kvStore.Close() }()

			if status, _ := cmd.Flags().GetBool("status"); status {
				st, err := migrate.Statuses(kvStore, migrate.Registered())
				if err != nil {
					return err
				}
				fmt.Printf("layout_version: %d\n", st.Layout)
				for _, m := range st.Migrations {
					fmt.Printf("  %d %-16s expanded=%t contracted=%t\n", m.Version, m.Name, m.Expanded, m.Contracted)
				}
				return nil
			}

			phase, _ := cmd.Flags().GetString("phase")
			if phase != string(migrate.PhaseExpand) && phase != string(migrate.PhaseContract) {
				return errors.New("--phase must be expand or contract")
			}
			to, _ := cmd.Flags().GetInt("to")
			dry, _ := cmd.Flags().GetBool("dry-run")

			stores := &migrate.Stores{KV: kvStore, Blob: blobStore}
			res, err := migrate.Run(cmd.Context(), stores, migrate.Registered(), migrate.RunOptions{
				Phase: migrate.Phase(phase), To: to, DryRun: dry,
			})
			slog.Info("migration run", "phase", res.Phase, "applied", res.Applied, "skipped", res.Skipped, "dryRun", dry)
			return err
		},
	}
	cmd.Flags().String("phase", "", "migration phase: expand (additive) or contract (cleanup)")
	cmd.Flags().Int("to", 0, "run up to and including this version (0 = all)")
	cmd.Flags().Bool("dry-run", false, "log the plan without mutating anything")
	cmd.Flags().Bool("status", false, "print the layout version and per-migration state")
	return cmd
}
