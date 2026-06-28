package repocmd

import (
	"github.com/spf13/cobra"
)

// Cmd groups the commands that publish to and prune the distribution repo
// on ayato. The verbs mirror Arch's repo-add / repo-remove: `repo add` uploads
// built package files, `repo remove` takes them back out.
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
