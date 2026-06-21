package cmd

import (
	"os"
	"os/exec"
	"path"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/spf13/cobra"
)

// srcinfoCmd regenerates the .SRCINFO files of the source packages. With no
// argument it covers every configured repository; pass a repo name to limit it
// to one.
func srcinfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "srcinfo [repo]",
		Aliases: []string{"us"},
		Short:   "Regenerate .SRCINFO files",
		Long:    "Regenerate .SRCINFO files for the source packages in a repository.",
		Args:    cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return getSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			dirs := make([]string, 0, len(config.Repos))
			if len(args) > 0 {
				d := getSrcDir(args[0])
				if d == "" {
					return utils.NewErr("invalid repository name: " + args[0])
				}
				dirs = append(dirs, d)
			} else {
				for _, r := range config.Repos {
					dirs = append(dirs, r.Dir)
				}
			}

			for _, dir := range dirs {
				srcdirs, err := repo.GetSrcDirs(dir)
				if err != nil {
					return err
				}
				for _, d := range srcdirs {
					gencmd := exec.Command("makepkg", "--printsrcinfo")
					gencmd.Dir = d

					srcinfoPath := path.Join(d, ".SRCINFO")
					srcinfoFile, err := os.Create(srcinfoPath)
					if err != nil {
						return err
					}

					gencmd.Stdout = srcinfoFile
					gencmd.Stderr = cmd.ErrOrStderr()
					err = gencmd.Run()
					// Close every iteration; otherwise one fd leaks per source dir.
					if cerr := srcinfoFile.Close(); err == nil {
						err = cerr
					}
					if err != nil {
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

func init() {
	subCmds.Add(srcinfoCmd())
}
