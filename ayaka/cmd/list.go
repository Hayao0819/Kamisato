package cmd

import (
	"log/slog"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/spf13/cobra"
)

// listCmd returns the command to list packages in the source repository.
// Returns the command to display a list of packages in the source repository.
func listCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "list",
		Short: "Display a list of packages in the source repository",
		Long:  "Displays a list of packages in the source repository.",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {

			repos := []*repo.SourceRepo{}
			if len(args) > 0 {
				argrepo := getSrcRepo(args[0])
				if argrepo == nil {
					return utils.NewErr("invalid repository name: " + args[0])
				}
				repos = append(repos, argrepo)
			} else {
				repos = srcRepo
			}

			for _, repo := range repos {

				for _, pkg := range repo.Pkgs {
					srcinfo, err := pkg.SRCINFO()
					if err != nil {
						slog.Warn("failed to get srcinfo", "error", err)
						continue
					}
					cmd.Println(srcinfo.PkgBase)
				}
			}

			return nil
		},
	}

	return &cmd
}

func init() {
	subCmds.Add(listCmd())
}
