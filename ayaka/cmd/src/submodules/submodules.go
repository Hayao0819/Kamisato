package submodulescmd

import (
	"log/slog"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/sourcerepos"
	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/vcs/git"
	"github.com/spf13/cobra"
)

func newCommand(sources sourcerepos.Reader) *cobra.Command {
	var (
		init      bool
		recursive bool
	)

	cmd := &cobra.Command{}
	if sources == nil {
		sources = sourcerepos.ForCommand(cmd)
	}
	*cmd = cobra.Command{
		Use:     "submodules",
		Aliases: []string{"update-submodules", "usm"},
		Short:   "Check out git submodules at their recorded commits",
		Long:    "Sync all git submodules in the repository directories to the commits the parent records; advancing a mirror to its origin is 'ayaka src pull'.",
		Args:    cmdline.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repos, err := sources.All()
			if err != nil {
				return err
			}
			for _, r := range repos {
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

func Cmd() *cobra.Command { return newCommand(nil) }
