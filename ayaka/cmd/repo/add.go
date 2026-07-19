package repocmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
)

func repoAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <repo> <pkgfile>...",
		Short: "Add package files (*.pkg.tar.*) to a binary repository on ayato",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := shared.RepoClient(cmd)
			if err != nil {
				return err
			}
			repoName := args[0]
			files := args[1:]
			return api.UploadPackageFiles(cmd.Context(), repoName, files...)
		},
	}
	shared.AddRepoServerFlags(cmd)
	return cmd
}
