package aurcmd

import (
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/gitcmd"
)

var aurPkgNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9@._+-]*$`)

func validateAurPkgName(name string) error {
	if !aurPkgNameRe.MatchString(name) {
		return errors.NewErrf("invalid AUR package name %q", name)
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

func aurMutationCmd(
	use, short, forceHelp string,
	run func(*cobra.Command, string, []string, bool) error,
) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.MinimumNArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return shared.AppFrom(cmd).GetSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, args[0], args[1:], force)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, forceHelp)
	return cmd
}

func runAurPackages(
	cmd *cobra.Command,
	repoName string,
	pkgs []string,
	errorPrefix string,
	run func(repoDir, name string) error,
) error {
	repo := shared.AppFrom(cmd).GetSrcRepo(repoName)
	if repo == nil {
		return errors.WrapErr(shared.ErrInvalidRepoName, repoName)
	}
	if repo.Dir == "" {
		return errors.WrapErr(shared.ErrNoSourceDir, repoName)
	}

	var failures []string
	for _, name := range pkgs {
		if err := run(repo.Dir, name); err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return errors.NewErr(errorPrefix + strings.Join(failures, "\n"))
	}
	return nil
}

func updateAurPkg(cobraCmd *cobra.Command, repoDir, name string, force bool) error {
	if err := validateAurPkgName(name); err != nil {
		return err
	}

	targetDir := filepath.Join(repoDir, name)
	gitDir := filepath.Join(targetDir, ".git")

	if _, err := os.Stat(gitDir); err == nil {
		slog.Info("updating existing AUR repo", "name", name, "dir", targetDir)
		if err := gitcmd.Pull(cobraCmd.Context(), targetDir); err != nil {
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

	root, err := shared.GitRootDir(repoDir)
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
		if err := gitcmd.AddSubmodule(cobraCmd.Context(), root, url, relPath); err != nil {
			return errors.WrapErr(err, "failed to add AUR submodule "+name)
		}
		return nil
	}

	slog.Info("cloning AUR repo (non-git parent)", "name", name, "dir", targetDir)
	if err := os.MkdirAll(repoDir, 0o755); err != nil { //nolint:gosec // G301: repo dir world-readable by design
		return errors.WrapErr(err, "failed to create repo directory")
	}
	if err := gitcmd.Clone(cobraCmd.Context(), gitcmd.CloneOptions{URL: url, Dir: targetDir, Strict: true}); err != nil {
		return errors.WrapErr(err, "failed to clone AUR package "+name)
	}
	return nil
}
