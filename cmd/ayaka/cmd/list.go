package cmd

import (
	"github.com/Hayao0819/ayaka/repo"
	"github.com/spf13/cobra"
)

func listCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "list",
		Short: "List packages in repository",
		Long:  "List packages in repository",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := repo.Get()
			if err != nil {
				return err
			}

			for _, pkg := range repo.Pkgs {
				cmd.Println(pkg.Srcinfo.Pkgbase)
			}

			return nil
		},
	}

	return &cmd
}

func init() {
	subCmds = append(subCmds, listCmd())
}
