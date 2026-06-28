package repocmd

import (
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
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

func repoAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <repo> <package_files...>",
		Short: "Add built packages to a repository on ayato",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := shared.RepoClient(cmd)
			if err != nil {
				return err
			}
			repoName := args[0]
			files := args[1:]
			return client.UploadPackageFiles(repoName, files...)
		},
	}
	shared.AddRepoServerFlags(cmd)
	return cmd
}

func repoRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <repo> <packages...>",
		Short: "Remove packages from a repository on ayato",
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
