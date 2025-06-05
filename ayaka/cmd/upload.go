package cmd

import (
	"errors"
	"log/slog"

	"github.com/Hayao0819/Kamisato/utils/blinkyutils"
	"github.com/spf13/cobra"
)

func uploadCmd() *cobra.Command {
	var server string
	cmd := cobra.Command{
		Use:  "upload server",
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			reponame := args[0]
			pkgs := args[1:]

			slog.Debug("uploading to blinky", "repo", reponame)

			if server == "" {
				return errors.New("server is required")
			}

			for _, pkg := range pkgs {
				blinkyutils.UploadToBlinky(server, reponame, pkg)
			}

			return nil

		},
	}

	cmd.Flags().StringVarP(&server, "server", "s", "", "Blinky server to upload to")
	return &cmd
}

func init() {
	subCmds = append(subCmds, uploadCmd())
}

// func UploadToBlinky(server string) error {
// 	client, err := blinkyutils.GetClient(server)
// 	if err != nil {
// 		return err
// 	}

// 	fp, err := p.GetPkgFileNames()
// 	if err != nil {
// 		return err
// 	}

// 	fullpaths := make([]string, len(fp))
// 	for i, f := range fp {
// 		fullpaths[i] = path.Join(p.Path, f)
// 	}

// 	return client.UploadPackageFiles(repo.Config.Name, fullpaths...)
// }
