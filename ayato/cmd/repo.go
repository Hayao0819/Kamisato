package cmd

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func repoCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "repo", Short: "Repository maintenance"}
	cmd.AddCommand(repoGCCmd())
	return cmd
}

// repoGCCmd reconciles a repo's storage against its pacman database, reporting
// (and with --delete removing) package objects the db does not reference — the
// residue of a presigned upload PUT but never finalized. Run it as a one-shot job.
func repoGCCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gc <repo>",
		Short: "Report orphan package objects not referenced by the repo db; --delete removes them",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoName := args[0]
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			cfg, err := conf.LoadAyatoConfig(cmd.Flags(), configFile)
			if err != nil {
				return err
			}
			cliutil.Setup(slog.LevelInfo, cliutil.ColorEnabled(cmd))

			pkgNameRepo, pkgBinaryRepo, authRepo, kvStore, err := repository.New(cfg)
			if err != nil {
				return errors.WrapErr(err, "failed to initialize repository")
			}
			defer func() { _ = kvStore.Close() }()

			signerRepo := repository.NewSignerRepository(kvStore)
			s := service.New(pkgNameRepo, pkgBinaryRepo, authRepo, signerRepo, cfg)

			olderThan, err := cmd.Flags().GetDuration("older-than")
			if err != nil {
				return err
			}
			if olderThan < 0 {
				return fmt.Errorf("--older-than must not be negative")
			}
			del, err := cmd.Flags().GetBool("delete")
			if err != nil {
				return err
			}

			orphans, err := s.ReconcileOrphans(repoName, olderThan, !del)
			if err != nil {
				return err
			}

			fmt.Printf("orphan objects: %d\n", len(orphans))
			for _, o := range orphans {
				action := "dry-run"
				if del {
					action = "deleted"
				}
				fmt.Printf("  %s/%s (age %s) [%s]\n", o.Arch, o.Name, o.Age.Round(time.Second), action)
			}
			if !del && len(orphans) > 0 {
				fmt.Println("re-run with --delete to remove them")
			}
			return nil
		},
	}
	cmd.Flags().Duration("older-than", time.Hour, "only consider objects at least this old (skips a fresh PUT mid-finalize)")
	cmd.Flags().Bool("delete", false, "delete the orphan objects (default: report only)")
	return cmd
}
