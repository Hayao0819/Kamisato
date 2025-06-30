package cmd

import (
	"log/slog"

	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/spf13/cobra"
)

func listCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "list",
		Short: "List packages in source repository",
		Long:  "List packages in source repository",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := repo.GetSrcRepo(config.RepoDir)
			if err != nil {
				return err
			}

			for _, pkg := range repo.Pkgs {
				srcinfo, err := pkg.SRCINFO()
				if err != nil {
					slog.Warn("failed to get srcinfo", "error", err)
					continue
				}
				cmd.Println(srcinfo.PkgBase)
			}

			return nil
		},
	}

	return &cmd
}

func init() {
	subCmds.Add(listCmd())
}
