package cmd

import (
	"log/slog"

	"github.com/Hayao0819/Kamisato/repo"
	"github.com/spf13/cobra"
)

func uploadCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:  "upload server",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			server := args[0]

			dest, err := repo.GetRepository(config.DestDir)
			if err != nil {
				return err
			}

			slog.Debug("uploading to blinky", "server", server, "dest", dest.Config.Name)

			return nil

		},
	}
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
