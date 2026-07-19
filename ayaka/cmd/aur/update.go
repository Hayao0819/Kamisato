package aurcmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

func aurUpdateCmd() *cobra.Command {
	return aurMutationCmd(
		"update <srcrepo> <pkgname>...",
		"Pull tracked AUR packages from upstream",
		"Force pull even on unclean state",
		runAurUpdate,
	)
}

func runAurUpdate(cmd *cobra.Command, repoName string, pkgs []string, force bool) error {
	return runAurPackages(cmd, repoName, pkgs, "one or more AUR updates failed:\n", func(repoDir, name string) error {
		gitDir := filepath.Join(repoDir, name, ".git")
		if _, err := os.Stat(gitDir); err != nil {
			return errors.NewErrf("package %q is not tracked; use 'aur add' to clone it first", name)
		}
		return updateAurPkg(cmd, repoDir, name, force)
	})
}
