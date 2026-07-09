package aurcmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func aurUpdateCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "update <srcrepo> <pkgname>...",
		Short: "Pull tracked AUR packages from upstream",
		Args:  cobra.MinimumNArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return shared.AppFrom(cmd).GetSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAurUpdate(cmd, args[0], args[1:], force)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force pull even on unclean state")
	return cmd
}

func runAurUpdate(cmd *cobra.Command, repoName string, pkgs []string, force bool) error {
	app := shared.AppFrom(cmd)
	if app.GetSrcRepo(repoName) == nil {
		return errors.WrapErr(shared.ErrInvalidRepoName, repoName)
	}
	repoDir := app.GetSrcDir(repoName)
	if repoDir == "" {
		return errors.WrapErr(shared.ErrNoSourceDir, repoName)
	}

	var errs []string
	for _, name := range pkgs {
		gitDir := filepath.Join(repoDir, name, ".git")
		if _, err := os.Stat(gitDir); err != nil {
			errs = append(errs, errors.NewErrf("package %q is not tracked; use 'aur add' to clone it first", name).Error())
			continue
		}
		if err := updateAurPkg(cmd, repoDir, name, force); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.NewErr("one or more AUR updates failed:\n" + strings.Join(errs, "\n"))
	}
	return nil
}
