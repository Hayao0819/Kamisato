package repocmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/buildclient"
)

func repoAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <repo> <pkgfile>...",
		Short: "Add package files (*.pkg.tar.*) to a binary repository on ayato",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := shared.ServerFromFlag(cmd)
			if err != nil {
				return err
			}
			repoName := args[0]
			files := args[1:]
			return shared.WithServerAuth(cmd.Context(), srv, func(ctx context.Context, token string) error {
				return buildclient.UploadPackageFiles(ctx, srv.URL, token, repoName, files...)
			})
		},
	}
	shared.AddRepoServerFlags(cmd)
	return cmd
}
