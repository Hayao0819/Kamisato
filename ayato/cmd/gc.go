package cmd

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayato/app"
	ayatoconfig "github.com/Hayao0819/Kamisato/ayato/config"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
)

func gcCmd() *cobra.Command {
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
			cfg, err := ayatoconfig.LoadAyatoConfig(cmd.Flags(), configFile)
			if err != nil {
				return err
			}
			cliutil.Setup(slog.LevelInfo, cliutil.ColorEnabled(cmd))

			olderThan, err := cmd.Flags().GetDuration("older-than")
			if err != nil {
				return err
			}
			del, err := cmd.Flags().GetBool("delete")
			if err != nil {
				return err
			}

			orphans, err := app.GC(cfg, repoName, olderThan, del)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "orphan objects: %d\n", len(orphans))
			for _, o := range orphans {
				action := "dry-run"
				if del {
					action = "deleted"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s/%s (age %s) [%s]\n", o.Arch, o.Name, o.Age.Round(time.Second), action)
			}
			if !del && len(orphans) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "re-run with --delete to remove them")
			}
			return nil
		},
	}
	cmd.Flags().Duration("older-than", time.Hour, "only consider objects at least this old (protects active publication)")
	cmd.Flags().Bool("delete", false, "delete the orphan objects (default: report only)")
	return cmd
}
