package aurcmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

func aurAddCmd() *cobra.Command {
	return aurMutationCmd(
		"add <srcrepo> <pkgname>...",
		"Clone AUR packages into a source repository (.ayakarc)",
		"Re-clone even if already tracked",
		runAurAdd,
	)
}

func runAurAdd(cmd *cobra.Command, repoName string, pkgs []string, force bool) error {
	return runAurPackages(cmd, repoName, pkgs, "one or more AUR adds failed:\n", func(repoDir, name string) error {
		gitDir := filepath.Join(repoDir, name, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			if !force {
				return errors.NewErrf("package %q is already tracked; use --force to re-clone", name)
			}
			if err := os.RemoveAll(filepath.Join(repoDir, name)); err != nil {
				return errors.WrapErr(err, "failed to remove "+name)
			}
		}
		return updateAurPkg(cmd, repoDir, name, false)
	})
}
