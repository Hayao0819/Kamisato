package submodulescmd

import (
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/vcs/git"
)

func Cmd(runtime *app.Runtime) *cobra.Command {
	var (
		init      bool
		recursive bool
	)

	cmd := &cobra.Command{
		Use:     "submodules",
		Aliases: []string{"update-submodules", "usm"},
		Short:   "Check out git submodules at their recorded commits",
		Long:    "Sync all git submodules in the repository directories to the commits the parent records; advancing a mirror to its origin is 'ayaka src pull'.",
		Args:    cmdline.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := runtime.App()
			if err != nil {
				return err
			}
			for _, r := range a.Config.Repos {
				root, err := git.RepoRoot(r.Dir)
				if err != nil {
					slog.Warn("skipping non-git repository", "dir", r.Dir)
					continue
				}

				slog.Info("updating submodules", "repo", root)

				if err := git.UpdateSubmodules(cmd.Context(), root, init, recursive); err != nil {
					return errors.WrapErr(err, "failed to update submodules in "+root)
				}

				cmd.Println("Updated submodules in:", root)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&init, "init", "i", false, "Initialize submodules before update")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Update submodules recursively")

	return cmd
}
