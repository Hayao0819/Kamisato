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
			slog.Warn("The upload command is under development. Please be careful.")
			slog.Warn("For stable operation, the blinky command is recommended.")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			reponame := args[0]
			pkgs := args[1:]

			slog.Debug("Start uploading to Blinky", "repo", reponame)

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

// Register the upload command as a subcommand

// func UploadToBlinky(server string) error {
