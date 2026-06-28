package aurcmd

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/spf13/cobra"
)

// Cmd groups the commands that pull PKGBUILDs from the AUR into a source repository.
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

// runAurFetch is the shared add/update path: a missing package is cloned, an
// existing one is pulled.
func runAurFetch(cmd *cobra.Command, repoName string, pkgs []string, aurBase string, force bool) error {
	if shared.GetSrcRepo(repoName) == nil {
		return utils.WrapErr(shared.ErrInvalidRepoName, repoName)
	}

	repoDir := shared.GetSrcDir(repoName)
	if repoDir == "" {
		return utils.WrapErr(shared.ErrNoSourceDir, repoName)
	}

	var errs []string
	for i, name := range pkgs {
		slog.Info("AUR PKGBUILD fetch", "index", i+1, "total", len(pkgs), "name", name)
		if err := updateAurPkg(cmd, repoDir, aurBase, name, force); err != nil {
			slog.Error("failed to fetch AUR package", "name", name, "error", err)
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return utils.NewErr("one or more AUR fetches failed:\n" + strings.Join(errs, "\n"))
	}
	return nil
}

func updateAurPkg(cobraCmd *cobra.Command, repoDir, aurBase, name string, force bool) error {
	targetDir := filepath.Join(repoDir, name)
	gitDir := filepath.Join(targetDir, ".git")

	if _, err := os.Stat(gitDir); err == nil {
		slog.Info("updating existing AUR repo", "name", name, "dir", targetDir)
		gitcmd := exec.Command("git", "-C", targetDir, "pull", "--ff-only")
		gitcmd.Stdout = cobraCmd.OutOrStdout()
		gitcmd.Stderr = cobraCmd.ErrOrStderr()
		if err := gitcmd.Run(); err != nil {
			return utils.WrapErr(err, "failed to pull AUR package "+name)
		}
		return nil
	}

	// force only matters for an existing non-git directory we want to re-fetch.
	if force {
		slog.Info("force remove non-git directory", "name", name, "dir", targetDir)
		if err := os.RemoveAll(targetDir); err != nil {
			return utils.WrapErr(err, "failed to remove existing directory for "+name)
		}
	}

	url := aurBase + "/" + name + ".git"

	root, err := shared.GitRootDir(repoDir)
	if err == nil {
		if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
			return utils.WrapErr(err, "failed to create parent directory")
		}

		relPath, err := filepath.Rel(root, targetDir)
		if err != nil {
			return utils.WrapErr(err, "failed to get relative path for submodule")
		}

		slog.Info("adding AUR repo as submodule", "name", name, "root", root, "path", relPath)
		gitcmd := exec.Command("git", "-C", root, "submodule", "add", url, relPath)
		gitcmd.Stdout = cobraCmd.OutOrStdout()
		gitcmd.Stderr = cobraCmd.ErrOrStderr()
		if err := gitcmd.Run(); err != nil {
			return utils.WrapErr(err, "failed to add AUR submodule "+name)
		}
		return nil
	}

	slog.Info("cloning AUR repo (non-git parent)", "name", name, "dir", targetDir)
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		return utils.WrapErr(err, "failed to create repo directory")
	}
	gitcmd := exec.Command("git", "clone", url, targetDir)
	gitcmd.Stdout = cobraCmd.OutOrStdout()
	gitcmd.Stderr = cobraCmd.ErrOrStderr()
	if err := gitcmd.Run(); err != nil {
		return utils.WrapErr(err, "failed to clone AUR package "+name)
	}
	return nil
}
