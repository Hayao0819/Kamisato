package aurcmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/spf13/cobra"
)

func aurAddCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "add <srcrepo> <pkgname>...",
		Short: "Clone AUR packages into a source repository (.ayakarc)",
		Args:  cobra.MinimumNArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return shared.AppFrom(cmd).GetSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAurAdd(cmd, args[0], args[1:], force)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Re-clone even if already tracked")
	return cmd
}

func runAurAdd(cmd *cobra.Command, repoName string, pkgs []string, force bool) error {
	app := shared.AppFrom(cmd)
	if app.GetSrcRepo(repoName) == nil {
		return errwrap.WrapErr(shared.ErrInvalidRepoName, repoName)
	}
	repoDir := app.GetSrcDir(repoName)
	if repoDir == "" {
		return errwrap.WrapErr(shared.ErrNoSourceDir, repoName)
	}

	var errs []string
	for _, name := range pkgs {
		gitDir := filepath.Join(repoDir, name, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			if !force {
				errs = append(errs, errwrap.NewErrf("package %q is already tracked; use --force to re-clone", name).Error())
				continue
			}
			if err := os.RemoveAll(filepath.Join(repoDir, name)); err != nil {
				errs = append(errs, errwrap.WrapErr(err, "failed to remove "+name).Error())
				continue
			}
		}
		if err := updateAurPkg(cmd, repoDir, name, false); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return errwrap.NewErr("one or more AUR adds failed:\n" + strings.Join(errs, "\n"))
	}
	return nil
}
