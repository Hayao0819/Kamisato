package srcinfocmd

import (
	"os"
	"os/exec"
	"path"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/spf13/cobra"
)

// Cmd regenerates .SRCINFO files; with no argument it covers every configured repository.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "srcinfo [repo]",
		Aliases: []string{"us"},
		Short:   "Regenerate .SRCINFO files",
		Long:    "Regenerate .SRCINFO files for the source packages in a repository.",
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
					return errwrap.WrapErr(shared.ErrInvalidRepoName, args[0])
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
					return errwrap.WrapErr(err, "failed to list source directories in "+dir)
				}
				for _, d := range srcdirs {
					gencmd := exec.Command("makepkg", "--printsrcinfo")
					gencmd.Dir = d

					srcinfoPath := path.Join(d, ".SRCINFO")
					srcinfoFile, err := os.Create(srcinfoPath)
					if err != nil {
						return errwrap.WrapErr(err, "failed to create .SRCINFO in "+d)
					}

					gencmd.Stdout = srcinfoFile
					gencmd.Stderr = cmd.ErrOrStderr()
					err = gencmd.Run()
					// Close every iteration; otherwise one fd leaks per source dir.
					if cerr := srcinfoFile.Close(); err == nil {
						err = cerr
					}
					if err != nil {
						return errwrap.WrapErr(err, "failed to generate .SRCINFO in "+d)
					}
					cmd.Println("Updated SRCINFO file:", d)
				}
			}
			return nil
		},
	}

	return cmd
}
