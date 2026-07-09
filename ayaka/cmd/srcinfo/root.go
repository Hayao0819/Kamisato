package srcinfocmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// Cmd regenerates .SRCINFO files; with no argument it covers every configured repository.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "srcinfo [<srcrepo>]",
		Aliases: []string{"us"},
		Short:   "Regenerate .SRCINFO files in a source repository (.ayakarc)",
		Long:    "Regenerate .SRCINFO files for the source packages in a source repository (.ayakarc).",
		Args:    cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return shared.AppFrom(cmd).GetSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			app := shared.AppFrom(cmd)
			dirs := make([]string, 0, len(app.Config.Repos))
			if len(args) > 0 {
				d := app.GetSrcDir(args[0])
				if d == "" {
					return errors.WrapErr(shared.ErrInvalidRepoName, args[0])
				}
				dirs = append(dirs, d)
			} else {
				for _, r := range app.Config.Repos {
					dirs = append(dirs, r.Dir)
				}
			}

			for _, dir := range dirs {
				srcdirs, err := repo.GetSrcDirs(dir)
				if err != nil {
					return errors.WrapErr(err, "failed to list source directories in "+dir)
				}
				for _, d := range srcdirs {
					if err := repo.GenerateSrcinfo(d, cmd.ErrOrStderr()); err != nil {
						return err
					}
					cmd.Println("Updated SRCINFO file:", d)
				}
			}
			return nil
		},
	}

	return cmd
}
