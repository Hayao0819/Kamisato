package cmd

import (
	"github.com/Hayao0819/ayaka/abs"
	"github.com/Hayao0819/ayaka/repo"
	"github.com/spf13/cobra"
)

func buildCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "build",
		Short: "Build packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := repo.Get()
			if err != nil {
				return err
			}
			builder := abs.Target{
				Arch: "x86_64",
			}

			return repo.Build(&builder)
		},
	}

	return &cmd
}

func init() {
	subCmds = append(subCmds, buildCmd())
}
