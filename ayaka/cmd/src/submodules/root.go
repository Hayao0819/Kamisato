package submodulescmd

import (
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/gitcmd"
)

func Cmd() *cobra.Command {
	var (
		init      bool
		recursive bool
		remote    bool
	)

	cmd := &cobra.Command{
		Use:     "submodules",
		Aliases: []string{"update-submodules", "usm"},
		Short:   "Update git submodules in the repository",
		Long:    "Pull and update all git submodules in the repository directories.",
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, r := range app.From(cmd).Config.Repos {
				root, err := gitcmd.RepoRoot(r.Dir)
				if err != nil {
					slog.Warn("skipping non-git repository", "dir", r.Dir)
					continue
				}

				slog.Info("updating submodules", "repo", root)

				if err := gitcmd.UpdateSubmodules(cmd.Context(), root, init, recursive, remote); err != nil {
					return errors.WrapErr(err, "failed to update submodules in "+root)
				}

				cmd.Println("Updated submodules in:", root)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&init, "init", "i", false, "Initialize submodules before update")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Update submodules recursively")
	cmd.Flags().BoolVarP(&remote, "remote", "", false, "Update submodules to latest commit on remote branch")

	return cmd
}
