package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayato/app"
	ayatoconfig "github.com/Hayao0819/Kamisato/ayato/config"
	"github.com/Hayao0819/Kamisato/ayato/migrate"
	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// migrateCmd runs a migration as a one-shot job. Run it separately (e.g. a Cloud Run
// Job), never inside the serving service, which throttles CPU and caps request time.
func migrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run data-layout migrations as a one-shot job",
		Args:  cmdline.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			cfg, err := ayatoconfig.LoadAyatoConfig(cmd.Flags(), configFile)
			if err != nil {
				return err
			}
			cmdline.Setup(slog.LevelInfo, cmdline.ColorEnabled(cmd))

			// K_SERVICE/K_REVISION mark a Cloud Run service; a Job has neither.
			if ayatoconfig.UnderCloudRun() {
				slog.Warn("ayato migrate is running inside a Cloud Run service, not a Job; run it as a Cloud Run Job — a service throttles CPU outside requests and caps requests at 60 minutes")
			}

			status, _ := cmd.Flags().GetBool("status")
			phase, _ := cmd.Flags().GetString("phase")
			if !status && phase != string(migrate.PhaseExpand) && phase != string(migrate.PhaseContract) {
				return errors.New("--phase must be expand or contract")
			}
			to, _ := cmd.Flags().GetInt("to")
			dry, _ := cmd.Flags().GetBool("dry-run")
			res, err := app.Migrate(cmd.Context(), cfg, status, migrate.RunOptions{
				Phase: migrate.Phase(phase), To: to, DryRun: dry,
			})
			if status {
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "layout_version: %d\n", res.Status.Layout)
				for _, m := range res.Status.Migrations {
					fmt.Fprintf(cmd.OutOrStdout(), "  %d %-16s expanded=%t contracted=%t\n", m.Version, m.Name, m.Expanded, m.Contracted)
				}
				return nil
			}

			slog.Info("migration run", "phase", res.Run.Phase, "applied", res.Run.Applied, "skipped", res.Run.Skipped, "dryRun", dry)
			return err
		},
	}
	cmd.Flags().String("phase", "", "migration phase: expand (additive) or contract (cleanup)")
	cmd.Flags().Int("to", 0, "run up to and including this version (0 = all)")
	cmd.Flags().Bool("dry-run", false, "log the plan without mutating anything")
	cmd.Flags().Bool("status", false, "print the layout version and per-migration state")
	return cmd
}
