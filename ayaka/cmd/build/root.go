package buildcmd

import (
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"slices"

	"github.com/Hayao0819/Kamisato/ayaka/build"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/pkg/pacman/alpm"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	"github.com/Hayao0819/Kamisato/pkg/pacman/gpg"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/spf13/cobra"
)

// Cmd builds packages locally in a clean chroot. Use `ayaka miko build` to
// submit to the remote miko build service.
func Cmd() *cobra.Command {
	var sign bool
	var gpgkey string
	var diffMode bool
	var repo string
	var executor string
	var arch string
	var updateSrcinfo bool
	cmd := cobra.Command{
		Use:   "build <srcrepo> [pkgname...]",
		Short: "Build packages locally (--diff to build only changed packages)",
		Args:  cobra.MinimumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			app := shared.AppFrom(cmd)
			if len(args) == 0 {
				return app.GetSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
			}

			repoName := args[0]
			sr := app.GetSrcRepo(repoName)
			if sr == nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			var cands []string
			for _, p := range sr.Pkgs {
				cands = append(cands, p.Base())
				cands = append(cands, p.Names()...)
			}

			return cands, cobra.ShellCompDirectiveNoFileComp
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			repo = args[0]

			if !slices.Contains(shared.AppFrom(cmd).GetSrcRepoNames(), repo) {
				return errwrap.WrapErr(shared.ErrInvalidRepoName, repo)
			}

			if !sign || diffMode {
				return nil
			}
			if gpgkey == "" {
				return errwrap.NewErr("--sign requires --key <gpg-key-id>")
			}
			slog.Info("Verifying GPG key", "key", gpgkey)
			tmpDir, err := os.MkdirTemp("", "ayaka-")
			if err != nil {
				return errwrap.WrapErr(err, "failed to create temporary directory")
			}
			defer os.RemoveAll(tmpDir)
			dummyFile := path.Join(tmpDir, "dummy.txt")
			if err := os.WriteFile(dummyFile, []byte("dummy"), 0o600); err != nil {
				return errwrap.WrapErr(err, "failed to create dummy file")
			}
			if err := gpg.SignFile(gpgkey, "", dummyFile); err != nil {
				return errwrap.WrapErr(err, "failed to sign dummy file")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			server, err := cmd.Flags().GetString("server")
			if err != nil {
				return err
			}
			var buildPkgs []string
			if len(args) > 1 {
				buildPkgs = args[1:]
			}

			app := shared.AppFrom(cmd)
			srcrepo := app.GetSrcRepo(repo)
			if srcrepo == nil {
				return errwrap.WrapErr(shared.ErrSourceRepoNotFound, repo)
			}
			destDir := app.GetDestDir(repo)
			if destDir == "" {
				return errwrap.WrapErr(shared.ErrNoDestDir, repo)
			}
			srcdir := app.GetSrcDir(repo)
			if srcdir == "" {
				return errwrap.WrapErr(shared.ErrNoSourceDir, repo)
			}

			// Keep .SRCINFO in sync with PKGBUILD before building: the source
			// versions read here (and the diff comparison below) come from .SRCINFO,
			// so a stale one would build or skip the wrong packages. Needs makepkg,
			// which a container/CI host may lack — warn and carry on there.
			if updateSrcinfo {
				if _, lookErr := exec.LookPath("makepkg"); lookErr != nil {
					slog.Warn("skipping .SRCINFO update: makepkg not found on PATH", "error", lookErr)
				} else {
					srcdirs, err := pacmanrepo.GetSrcDirs(srcdir)
					if err != nil {
						return errwrap.WrapErr(err, "failed to list source directories")
					}
					for _, d := range srcdirs {
						if err := pacmanrepo.GenerateSrcinfo(d, cmd.ErrOrStderr()); err != nil {
							slog.Warn("failed to update .SRCINFO", "dir", d, "error", err)
						}
					}
					// Reload so the freshly written versions drive the diff and build.
					reloaded, err := pacmanrepo.GetSrcRepo(srcdir, srcrepo.Config)
					if err != nil {
						return errwrap.WrapErr(err, "failed to reload source repo after .SRCINFO update")
					}
					srcrepo = reloaded
				}
			}

			pkgs, cleanup, err := alpm.GetCleanPkgBinary(srcrepo.Config.InstallPkgs.Names...)
			if err != nil {
				return errwrap.WrapErr(err, "failed to get clean package binaries")
			}
			defer func() { _ = cleanup.Close() }()

			slog.Info("Creating build target", "arch", srcrepo.Config.ArchBuild, "installpkgs", pkgs)

			var signKey string
			if sign {
				signKey = gpgkey
			}
			buildTarget := builder.Target{
				Arch:        arch,
				ArchBuild:   srcrepo.Config.ArchBuild,
				SignKey:     signKey,
				InstallPkgs: append(srcrepo.Config.InstallPkgs.Files, pkgs...),
				Executor:    builder.Kind(executor),
			}

			if server == "" {
				server = srcrepo.Config.Server
			}
			outDir := path.Join(destDir, srcrepo.Config.Name)
			writeDir := path.Join(outDir, buildTarget.Arch)

			if diffMode {
				slog.Info("Starting diff build", "repo", srcdir, "outdir", writeDir, "gpgkey", gpgkey, "server", server)
				remoteRepo, err := pacmanrepo.RepoFromURL(server, srcrepo.Config.Name)
				if err != nil {
					if errors.Is(err, pacmanrepo.ErrRepoNotFound) {
						return errwrap.WrapErr(err, "remote repo not found; configure it on ayato and point --server at the repo db dir (.../repo/<repo>/<arch>)")
					}
					return errwrap.WrapErr(err, "failed to get remote repository")
				}
				if err := build.Diff(srcrepo, &buildTarget, remoteRepo, outDir, buildPkgs...); err != nil {
					return errwrap.WrapErr(err, "failed to perform diff build")
				}
				slog.Debug("Diff build completed", "outdir", writeDir)
				return nil
			}

			slog.Info("Starting package build", "repo", srcdir, "outdir", writeDir, "gpgkey", gpgkey)
			if err := build.Repo(srcrepo, &buildTarget, outDir, buildPkgs...); err != nil {
				return errwrap.WrapErr(err, "failed to build package")
			}
			slog.Debug("Build completed", "outdir", writeDir)
			return nil
		},
	}
	cmd.Flags().BoolVar(&sign, "sign", false, "Sign built packages with the GPG key specified by --key")
	cmd.Flags().StringVar(&gpgkey, "key", "", "GPG key ID for package signing (requires --sign)")
	cmd.Flags().BoolVar(&diffMode, "diff", false, "Enable diff build mode (build only new packages)")
	shared.AddServerFlag(&cmd)
	cmd.Flags().StringVar(&executor, "executor", "chroot", "Local build backend: chroot or container")
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Target architecture for the build")
	cmd.Flags().BoolVar(&updateSrcinfo, "update-srcinfo", true, "Regenerate .SRCINFO from PKGBUILD before building (requires makepkg; skipped when absent)")
	return &cmd
}
