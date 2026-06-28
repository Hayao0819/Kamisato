package repocmd

import (
	"github.com/spf13/cobra"
)

// Cmd groups the commands that publish to and prune the ayato distribution repo,
// mirroring Arch's repo-add / repo-remove.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Publish packages to the distribution repository on ayato",
		Long:  "Add built packages to, or remove them from, a repository served by ayato.",
	}
	cmd.AddCommand(
		repoAddCmd(),
		repoRemoveCmd(),
	)
	return cmd
}
