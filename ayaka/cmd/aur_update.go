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

// aurUpdateCmd updates local PKGBUILD git trees cloned from AUR.
// Usage: ayaka aur-update <repo> <aur-pkg> [aur-pkg...]
//
// 実装方針は yay の PKGBUILD 取得ロジックを参考にしつつ、
// 指定した Ayaka リポジトリ配下に AUR の git リポジトリを
// clone / pull するシンプルなものにしています。
func aurUpdateCmd() *cobra.Command {
	var (
		force   bool
		aurBase = "https://aur.archlinux.org"
	)

	cmd := &cobra.Command{
		Use:   "aur-update <repo> <aur-pkg> [aur-pkg...]",
		Short: "Update PKGBUILD repositories from AUR",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoName := args[0]
			if getSrcRepo(repoName) == nil {
				return utils.NewErr("invalid repository name: " + repoName)
			}

			repoDir := getSrcDir(repoName)
			if repoDir == "" {
				return utils.NewErr("failed to get source directory")
			}

			pkgs := args[1:]
			var errs []string
			for i, name := range pkgs {
				slog.Info("AUR PKGBUILD update", "index", i+1, "total", len(pkgs), "name", name)
				if err := updateAurPkg(cmd, repoDir, aurBase, name, force); err != nil {
					slog.Error("failed to update AUR package", "name", name, "error", err)
					errs = append(errs, err.Error())
				}
			}

			if len(errs) > 0 {
				return utils.NewErr("one or more AUR updates failed:\n" + strings.Join(errs, "\n"))
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Re-clone repositories even if they already exist")
	return cmd
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
	subCmds = append(subCmds, aurUpdateCmd())
}
