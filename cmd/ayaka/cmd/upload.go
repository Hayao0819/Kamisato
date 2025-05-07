package cmd

import (
	"github.com/Hayao0819/ayaka/repo"
	"github.com/spf13/cobra"
)

func uploadCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:  "upload server",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			server := args[0]

			repo, err := repo.Get()
			if err != nil {
				return err
			}

			return repo.UploadAllPackageToBlinky(server)
		},
	}
	return &cmd
}

func init() {
	subCmds = append(subCmds, uploadCmd())
}
