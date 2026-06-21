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
		return utils.NewErr("invalid repository name: " + repoName)
	}

	repoDir := getSrcDir(repoName)
	if repoDir == "" {
		return utils.NewErr("failed to get source directory")
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
// 挙動:
//   - targetDir に .git があれば (通常の git repo / サブモジュール問わず)、git pull で更新
//   - そうでなければ、まず repoDir が git 管理下かを調べ、
//   - 管理下なら git submodule add で追加 (既定の挙動)
//   - 管理下でなければ git clone で取得
//   - force は「まだ git repo でないディレクトリを取り直す」用途に限定し、
//     既に git 管理下にある (submodule 含む) 場合は pull のみ行います。
func updateAurPkg(cobraCmd *cobra.Command, repoDir, aurBase, name string, force bool) error {
	targetDir := filepath.Join(repoDir, name)
	gitDir := filepath.Join(targetDir, ".git")

	// 既に git 管理下であれば、そのまま pull で更新する
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

	// git repo でない既存ディレクトリを取り直したい場合だけ削除する
	if force {
		slog.Info("force remove non-git directory", "name", name, "dir", targetDir)
		if err := os.RemoveAll(targetDir); err != nil {
			return utils.WrapErr(err, "failed to remove existing directory for "+name)
		}
	}

	url := aurBase + "/" + name + ".git"

	// repoDir が git 管理下であれば、既定では submodule として追加する
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

	// repoDir が git 管理下でない場合のみ、通常の git clone を使う
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
