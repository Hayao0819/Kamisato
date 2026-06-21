package cmd

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/spf13/cobra"
)

// aurCmd groups the commands that pull PKGBUILDs from the AUR into a local
// source repository: `aur add` to take in a new package and `aur update` to
// follow upstream changes.
func aurCmd() *cobra.Command {
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

// aurAddCmd clones one or more AUR packages into a source repository for the
// first time. When the package is already present it falls back to a pull, so
// the command is safe to re-run.
func aurAddCmd() *cobra.Command {
	var force bool
	const aurBase = "https://aur.archlinux.org"

	cmd := &cobra.Command{
		Use:   "add <repo> <aur-pkg> [aur-pkg...]",
		Short: "Add AUR packages to a source repository",
		Args:  cobra.MinimumNArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return getSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAurFetch(cmd, args[0], args[1:], aurBase, force)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Re-clone repositories even if they already exist")
	return cmd
}

// aurUpdateCmd pulls already-tracked AUR packages in a source repository.
func aurUpdateCmd() *cobra.Command {
	var force bool
	const aurBase = "https://aur.archlinux.org"

	cmd := &cobra.Command{
		Use:   "update <repo> <aur-pkg> [aur-pkg...]",
		Short: "Update tracked AUR packages from upstream",
		Args:  cobra.MinimumNArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return getSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		// TODO: update every tracked package when no package name is given.
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAurFetch(cmd, args[0], args[1:], aurBase, force)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Re-clone repositories even if they already exist")
	return cmd
}

// runAurFetch clones or updates each named AUR package under the given source
// repository. add and update share this path: a missing package is cloned, an
// existing one is pulled.
func runAurFetch(cmd *cobra.Command, repoName string, pkgs []string, aurBase string, force bool) error {
	if getSrcRepo(repoName) == nil {
		return utils.WrapErr(ErrInvalidRepoName, repoName)
	}

	repoDir := getSrcDir(repoName)
	if repoDir == "" {
		return utils.WrapErr(ErrNoSourceDir, repoName)
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

// updateAurPkg clones or updates a single AUR git repository under repoDir.
//
// Behavior:
//   - if targetDir has a .git (whether a normal git repo or a submodule), update with git pull
//   - otherwise, first check whether repoDir is under git control,
//   - if it is, add via git submodule add (the default behavior)
//   - if it is not, fetch with git clone
//   - force is limited to "re-fetching a directory that is not yet a git repo";
//     when already under git control (including submodules), only pull is performed.
func updateAurPkg(cobraCmd *cobra.Command, repoDir, aurBase, name string, force bool) error {
	targetDir := filepath.Join(repoDir, name)
	gitDir := filepath.Join(targetDir, ".git")

	// If already under git control, just update with pull
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

	// Only remove when we want to re-fetch an existing directory that is not a git repo
	if force {
		slog.Info("force remove non-git directory", "name", name, "dir", targetDir)
		if err := os.RemoveAll(targetDir); err != nil {
			return utils.WrapErr(err, "failed to remove existing directory for "+name)
		}
	}

	url := aurBase + "/" + name + ".git"

	// If repoDir is under git control, add as a submodule by default
	root, err := gitRootDir(repoDir)
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

	// Only use a normal git clone when repoDir is not under git control
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

// gitRootDir returns the root directory of the git repository that contains dir.
// If dir is not inside a git repository, an error is returned.
func gitRootDir(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func init() {
	subCmds.Add(aurCmd())
}
