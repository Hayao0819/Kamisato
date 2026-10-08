package gccmd

import (
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayato/config"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/service"
	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func Cmd() *cobra.Command {
	var olderThan time.Duration
	var deleteObjects bool
	cmd := &cobra.Command{
		Use:   "gc <repo>",
		Short: "Report orphan package objects not referenced by the repo db; --delete removes them",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (returnedErr error) {
			repoName := args[0]
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			cfg, err := config.LoadAyatoConfig(cmd.Flags(), configFile)
			if err != nil {
				return err
			}
			cmdline.Setup(slog.LevelInfo, cmdline.ColorEnabled(cmd))

			if olderThan < 0 {
				return errors.New("orphan age must not be negative")
			}
			settings, err := config.RepositorySettings(cfg)
			if err != nil {
				return err
			}
			nameStore, binaryRepo, authRepo, store, err := repository.New(settings)
			if err != nil {
				return errors.WrapErr(err, "failed to initialize repository")
			}
			defer func() { returnedErr = errors.Join(returnedErr, store.Close()) }()
			svc := service.New(nameStore, binaryRepo, authRepo, repository.NewSignerRepository(store), config.ServiceSettings(cfg))
			return run(cmd.OutOrStdout(), svc, repoName, olderThan, deleteObjects)
		},
	}
	cmd.Flags().DurationVar(&olderThan, "older-than", time.Hour, "only consider objects at least this old (protects active publication)")
	cmd.Flags().BoolVar(&deleteObjects, "delete", false, "delete the orphan objects (default: report only)")
	return cmd
}

type reconciler interface {
	ReconcileOrphans(string, time.Duration, bool) ([]service.OrphanObject, error)
}

func run(out io.Writer, svc reconciler, repoName string, olderThan time.Duration, deleteObjects bool) error {
	orphans, err := svc.ReconcileOrphans(repoName, olderThan, !deleteObjects)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "orphan objects: %d\n", len(orphans))
	for _, orphan := range orphans {
		action := "dry-run"
		if deleteObjects {
			action = "deleted"
		}
		fmt.Fprintf(out, "  %s/%s (age %s) [%s]\n", orphan.Arch, orphan.Name, orphan.Age.Round(time.Second), action)
	}
	if !deleteObjects && len(orphans) > 0 {
		fmt.Fprintln(out, "re-run with --delete to remove them")
	}
	return nil
}
