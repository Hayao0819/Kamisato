package repocmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
)

func repoRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <repo> <pkgname>...",
		Short: "Remove packages by name from a binary repository on ayato",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := shared.RepoClient(cmd)
			if err != nil {
				return err
			}
			repoName := args[0]
			pkgs := args[1:]
			return client.RemovePackages(repoName, pkgs...)
		},
	}
	shared.AddRepoServerFlags(cmd)
	return cmd
}
