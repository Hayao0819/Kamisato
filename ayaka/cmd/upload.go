package cmd

import (
	"errors"
	"log/slog"

	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/spf13/cobra"
)

func uploadCmd() *cobra.Command {
	var server string
	cmd := cobra.Command{
		Use:  "upload server",
		Args: cobra.MinimumNArgs(2),
		PreRun: func(cmd *cobra.Command, args []string) {
			slog.Warn("Upload command is still in development, use with caution")
			slog.Warn("Please use blinky command for stable usage")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			reponame := args[0]
			pkgs := args[1:]

			slog.Debug("uploading to blinky", "repo", reponame)

			if server == "" {
				return errors.New("server is required")
			}

			for _, pkg := range pkgs {
				_ = blinkyutils.UploadToBlinky(server, reponame, pkg)
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
