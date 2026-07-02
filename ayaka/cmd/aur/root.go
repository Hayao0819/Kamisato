package aurcmd

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/spf13/cobra"
)

// aurPkgNameRe is the AUR pkgbase charset. The leading [a-z0-9] forbids a leading
// dot or dash, and the charset has no "/", so a name can never be a path
// traversal ("../x") that escapes the source repo directory.
var aurPkgNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9@._+-]*$`)

func validateAurPkgName(name string) error {
	if !aurPkgNameRe.MatchString(name) {
		return errwrap.NewErrf("invalid AUR package name %q", name)
	}
	return nil
}

// aurBase is the AUR git host that pkgbase clones are pulled from.
const aurBase = "https://aur.archlinux.org"

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

// aurFetchCmd builds the add/update command: both clone a missing pkgbase and
// pull an existing one, differing only in wording, so they share one builder.
func aurFetchCmd(use, short string) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.MinimumNArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return shared.AppFrom(cmd).GetSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAurFetch(cmd, args[0], args[1:], force)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Re-clone repositories even if they already exist")
	return cmd
}

// runAurFetch is the shared add/update path: a missing package is cloned, an
// existing one is pulled.
func runAurFetch(cmd *cobra.Command, repoName string, pkgs []string, force bool) error {
	app := shared.AppFrom(cmd)
	if app.GetSrcRepo(repoName) == nil {
		return errwrap.WrapErr(shared.ErrInvalidRepoName, repoName)
	}

	repoDir := app.GetSrcDir(repoName)
	if repoDir == "" {
		return errwrap.WrapErr(shared.ErrNoSourceDir, repoName)
	}

	var errs []string
	for i, name := range pkgs {
		slog.Info("AUR PKGBUILD fetch", "index", i+1, "total", len(pkgs), "name", name)
		if err := updateAurPkg(cmd, repoDir, name, force); err != nil {
			slog.Error("failed to fetch AUR package", "name", name, "error", err)
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return errwrap.NewErr("one or more AUR fetches failed:\n" + strings.Join(errs, "\n"))
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
		gitcmd := exec.Command("git", "-C", targetDir, "pull", "--ff-only") //nolint:gosec // fixed program git, argv passed as separate args (no shell)
		gitcmd.Stdout = cobraCmd.OutOrStdout()
		gitcmd.Stderr = cobraCmd.ErrOrStderr()
		if err := gitcmd.Run(); err != nil {
			return errwrap.WrapErr(err, "failed to pull AUR package "+name)
		}
		return nil
	}

	// force only matters for an existing non-git directory we want to re-fetch.
	if force {
		slog.Info("force remove non-git directory", "name", name, "dir", targetDir)
		if err := os.RemoveAll(targetDir); err != nil {
			return errwrap.WrapErr(err, "failed to remove existing directory for "+name)
		}
	}

	url := aurBase + "/" + name + ".git"

	root, err := shared.GitRootDir(repoDir)
	if err == nil {
		if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil { //nolint:gosec // user's AUR repo workspace, world-readable by convention
			return errwrap.WrapErr(err, "failed to create parent directory")
		}

		relPath, err := filepath.Rel(root, targetDir)
		if err != nil {
			return errwrap.WrapErr(err, "failed to get relative path for submodule")
		}

		slog.Info("adding AUR repo as submodule", "name", name, "root", root, "path", relPath)
		gitcmd := exec.Command("git", "-C", root, "submodule", "add", url, relPath) //nolint:gosec // fixed program git, argv passed as separate args (no shell)
		gitcmd.Stdout = cobraCmd.OutOrStdout()
		gitcmd.Stderr = cobraCmd.ErrOrStderr()
		if err := gitcmd.Run(); err != nil {
			return errwrap.WrapErr(err, "failed to add AUR submodule "+name)
		}
		return nil
	}

	slog.Info("cloning AUR repo (non-git parent)", "name", name, "dir", targetDir)
	if err := os.MkdirAll(repoDir, 0o755); err != nil { //nolint:gosec // user's AUR repo workspace, world-readable by convention
		return errwrap.WrapErr(err, "failed to create repo directory")
	}
	gitcmd := exec.Command("git", "clone", url, targetDir) //nolint:gosec // fixed program git, argv passed as separate args (no shell)
	gitcmd.Stdout = cobraCmd.OutOrStdout()
	gitcmd.Stderr = cobraCmd.ErrOrStderr()
	if err := gitcmd.Run(); err != nil {
		return errwrap.WrapErr(err, "failed to clone AUR package "+name)
	}
	return nil
}
