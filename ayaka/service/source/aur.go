package source

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/gitcmd"
)

var aurPkgNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9@._+-]*$`)

const aurBase = "https://aur.archlinux.org"

// AddAUR clones (or re-clones with force) each named AUR package into repoDir,
// as a submodule when repoDir is inside a git repo. Failures are collected so
// one bad package does not stop the rest.
func AddAUR(ctx context.Context, repoDir string, names []string, force bool) error {
	return eachAUR("one or more AUR adds failed:\n", names, func(name string) error {
		gitDir := filepath.Join(repoDir, name, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			if !force {
				return errors.NewErrf("package %q is already tracked; use --force to re-clone", name)
			}
			if err := os.RemoveAll(filepath.Join(repoDir, name)); err != nil {
				return errors.WrapErr(err, "failed to remove "+name)
			}
		}
		return checkoutAUR(ctx, repoDir, name, false)
	})
}

// UpdateAUR pulls each tracked AUR package in repoDir from upstream.
func UpdateAUR(ctx context.Context, repoDir string, names []string, force bool) error {
	return eachAUR("one or more AUR updates failed:\n", names, func(name string) error {
		gitDir := filepath.Join(repoDir, name, ".git")
		if _, err := os.Stat(gitDir); err != nil {
			return errors.NewErrf("package %q is not tracked; use 'aur add' to clone it first", name)
		}
		return checkoutAUR(ctx, repoDir, name, force)
	})
}

func eachAUR(errorPrefix string, names []string, run func(name string) error) error {
	var failures []string
	for _, name := range names {
		if err := run(name); err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return errors.NewErr(errorPrefix + strings.Join(failures, "\n"))
	}
	return nil
}

// checkoutAUR materializes one AUR package: pull when already a git checkout,
// else add it as a submodule (git parent) or plain clone (non-git parent).
func checkoutAUR(ctx context.Context, repoDir, name string, force bool) error {
	if !aurPkgNameRe.MatchString(name) {
		return errors.NewErrf("invalid AUR package name %q", name)
	}

	targetDir := filepath.Join(repoDir, name)
	gitDir := filepath.Join(targetDir, ".git")

	if _, err := os.Stat(gitDir); err == nil {
		slog.Info("updating existing AUR repo", "name", name, "dir", targetDir)
		if err := gitcmd.Pull(ctx, targetDir); err != nil {
			return errors.WrapErr(err, "failed to pull AUR package "+name)
		}
		return nil
	}

	if force {
		slog.Info("force remove non-git directory", "name", name, "dir", targetDir)
		if err := os.RemoveAll(targetDir); err != nil {
			return errors.WrapErr(err, "failed to remove existing directory for "+name)
		}
	}

	url := aurBase + "/" + name + ".git"

	root, err := gitcmd.RepoRoot(repoDir)
	if err == nil {
		if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil { //nolint:gosec // G301: repo dir world-readable by design
			return errors.WrapErr(err, "failed to create parent directory")
		}

		absTarget, err := filepath.Abs(targetDir)
		if err != nil {
			return errors.WrapErr(err, "failed to resolve absolute target path")
		}

		relPath, err := filepath.Rel(root, absTarget)
		if err != nil {
			return errors.WrapErr(err, "failed to get relative path for submodule")
		}

		slog.Info("adding AUR repo as submodule", "name", name, "root", root, "path", relPath)
		if err := gitcmd.AddSubmodule(ctx, root, url, relPath); err != nil {
			return errors.WrapErr(err, "failed to add AUR submodule "+name)
		}
		return nil
	}

	slog.Info("cloning AUR repo (non-git parent)", "name", name, "dir", targetDir)
	if err := os.MkdirAll(repoDir, 0o755); err != nil { //nolint:gosec // G301: repo dir world-readable by design
		return errors.WrapErr(err, "failed to create repo directory")
	}
	if err := gitcmd.Clone(ctx, gitcmd.CloneOptions{URL: url, Dir: targetDir, Strict: true}); err != nil {
		return errors.WrapErr(err, "failed to clone AUR package "+name)
	}
	return nil
}
