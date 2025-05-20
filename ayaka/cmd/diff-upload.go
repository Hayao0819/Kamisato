package cmd

import (
	"github.com/Hayao0819/Kamisato/ayaka/repo"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"
)

func diffBuildCmd() *cobra.Command {
	var server string
	cmd := cobra.Command{
		Use:  "diff-build",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			srcrepo, err := repo.GetSrcRepo(config.RepoDir)
			if err != nil {
				return errors.Wrap(err, "failed to get source repository")
			}

			if server == "" {
				server = srcrepo.Config.Server
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&server, "server", "s", "", "Blinky server to upload to")

	return &cmd
}

func init() {
	subCmds.Add(utils.TodoCmd("diff-build"))
}
