package aurcmd

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/spf13/cobra"
)

var aurPkgNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9@._+-]*$`)

func validateAurPkgName(name string) error {
	if !aurPkgNameRe.MatchString(name) {
		return errwrap.NewErrf("invalid AUR package name %q", name)
	}
	return nil
}

const aurBase = "https://aur.archlinux.org"

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aur",
		Short: "Manage PKGBUILDs taken from the AUR",
		Long:  "Add AUR packages to a source repository and update them from upstream.",
	}
	cmd.AddCommand(
		aurAddCmd(),
		aurUpdateCmd(),
	)
	return cmd
}

func updateAurPkg(cobraCmd *cobra.Command, repoDir, name string, force bool) error {
	if err := validateAurPkgName(name); err != nil {
		return err
	}

	targetDir := filepath.Join(repoDir, name)
	gitDir := filepath.Join(targetDir, ".git")

	if _, err := os.Stat(gitDir); err == nil {
		slog.Info("updating existing AUR repo", "name", name, "dir", targetDir)
		gitcmd := exec.Command("git", "-C", targetDir, "pull", "--ff-only")
		gitcmd.Stdout = cobraCmd.OutOrStdout()
		gitcmd.Stderr = cobraCmd.ErrOrStderr()
		if err := gitcmd.Run(); err != nil {
			return errwrap.WrapErr(err, "failed to pull AUR package "+name)
		}
		return nil
	}

	if force {
		slog.Info("force remove non-git directory", "name", name, "dir", targetDir)
		if err := os.RemoveAll(targetDir); err != nil {
			return errwrap.WrapErr(err, "failed to remove existing directory for "+name)
		}
	}

	url := aurBase + "/" + name + ".git"

	root, err := shared.GitRootDir(repoDir)
	if err == nil {
		if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
			return errwrap.WrapErr(err, "failed to create parent directory")
		}

		relPath, err := filepath.Rel(root, targetDir)
		if err != nil {
			return errwrap.WrapErr(err, "failed to get relative path for submodule")
		}

		slog.Info("adding AUR repo as submodule", "name", name, "root", root, "path", relPath)
		gitcmd := exec.Command("git", "-C", root, "submodule", "add", url, relPath)
		gitcmd.Stdout = cobraCmd.OutOrStdout()
		gitcmd.Stderr = cobraCmd.ErrOrStderr()
		if err := gitcmd.Run(); err != nil {
			return errwrap.WrapErr(err, "failed to add AUR submodule "+name)
		}
		return nil
	}

	slog.Info("cloning AUR repo (non-git parent)", "name", name, "dir", targetDir)
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		return errwrap.WrapErr(err, "failed to create repo directory")
	}
	gitcmd := exec.Command("git", "clone", url, targetDir)
	gitcmd.Stdout = cobraCmd.OutOrStdout()
	gitcmd.Stderr = cobraCmd.ErrOrStderr()
	if err := gitcmd.Run(); err != nil {
		return errwrap.WrapErr(err, "failed to clone AUR package "+name)
	}
	return nil
}
