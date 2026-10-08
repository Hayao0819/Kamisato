package addcmd

import (
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/remote"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <repo> <pkgfile>...",
		Short: "Add package files (*.pkg.tar.*) to a binary repository on ayato",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := remote.DefaultProvider().RepoClient(cmd)
			if err != nil {
				return err
			}
			repoName := args[0]
			files := args[1:]
			return api.UploadPackageFiles(cmd.Context(), repoName, files...)
		},
	}
	remote.AddRepoServerFlags(cmd)
	return cmd
}
