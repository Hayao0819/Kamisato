package submodulescmd

import (
	"log/slog"
	"os/exec"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	var (
		init      bool
		recursive bool
		remote    bool
	)

	cmd := &cobra.Command{
		Use:     "update-submodules",
		Aliases: []string{"usm"},
		Short:   "Update git submodules in the repository",
		Long:    "Pull and update all git submodules in the repository directories.",
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, r := range shared.Config.Repos {
				root, err := shared.GitRootDir(r.Dir)
				if err != nil {
					slog.Warn("skipping non-git repository", "dir", r.Dir)
					continue
				}

				slog.Info("updating submodules", "repo", root)

				gitArgs := []string{"-C", root, "submodule", "update"}

				if init {
					gitArgs = append(gitArgs, "--init")
				}
				if recursive {
					gitArgs = append(gitArgs, "--recursive")
				}
				if remote {
					gitArgs = append(gitArgs, "--remote")
				}

				gitcmd := exec.Command("git", gitArgs...)
				gitcmd.Stdout = cmd.OutOrStdout()
				gitcmd.Stderr = cmd.ErrOrStderr()

				if err := gitcmd.Run(); err != nil {
					return utils.WrapErr(err, "failed to update submodules in "+root)
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
