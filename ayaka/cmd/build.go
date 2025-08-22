package cmd

import (
	"log/slog"
	"os"
	"path"
	"path/filepath"

	"github.com/Hayao0819/Kamisato/ayaka/gpg"
	"github.com/Hayao0819/Kamisato/internal/utils"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/package"
	"github.com/Hayao0819/Kamisato/pkg/pacman/package/builder"
	"github.com/Hayao0819/Kamisato/pkg/pacman/remote"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	pacman_utils "github.com/Hayao0819/Kamisato/pkg/pacman/utils"
	"github.com/spf13/cobra"
)

// buildCmd returns a unified build command for normal and diff-build modes.
// Returns the command to build packages (normal and diff-build).
func buildCmd() *cobra.Command {
	var gpgkey string
	var diffMode bool
	var server string
	cmd := cobra.Command{
		Use:   "build",
		Short: "Build packages (--diff for diff build)",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if gpgkey == "" || diffMode {
				return nil
			}
			slog.Info("Verifying GPG key", "key", gpgkey)
			tmpDir, err := os.MkdirTemp("", "ayaka-")
			if err != nil {
				return utils.WrapErr(err, "failed to create temporary directory")
			}
			defer os.RemoveAll(tmpDir)
			dummyFile := path.Join(tmpDir, "dummy.txt")
			if err := os.WriteFile(dummyFile, []byte("dummy"), 0644); err != nil {
				return utils.WrapErr(err, "failed to create dummy file")
			}
			if err := gpg.SignFile(gpgkey, "", dummyFile); err != nil {
				return utils.WrapErr(err, "failed to sign dummy file")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			destDir, err := filepath.Abs(config.DestDir)
			if err != nil {
				return utils.WrapErr(err, "failed to get absolute path for output directory")
			}
			repoDir, err := filepath.Abs(config.RepoDir)
			if err != nil {
				return utils.WrapErr(err, "failed to get absolute path for repository directory")
			}
			srcrepo, err := repo.GetSrcRepo(repoDir)
			if err != nil {
				return utils.WrapErr(err, "failed to get source repository")
			}

			// Diff build mode
			if diffMode {
				if server == "" {
					server = srcrepo.Config.Server
				}
				slog.Debug("Getting diff build info", "repo", srcrepo.Config.Name, "server", server)
				rr, err := remote.GetRepoFromURL(server, srcrepo.Config.Name)
				if err != nil {
					return utils.WrapErr(err, "failed to get remote repository")
				}
				var shoubuild []*pkg.Package
				for _, pkg := range srcrepo.Pkgs {
					pi := pkg.MustPKGINFO()
					rp := rr.PkgByPkgBase(pi.PkgBase)
					if rp == nil {
						slog.Warn("Package does not exist in remote repository", "pkgbase", pi.PkgBase)
						shoubuild = append(shoubuild, pkg)
						continue
					}
					cmp, err := pacman_utils.VerCmp(pi.PkgVer, rp.MustPKGINFO().PkgVer)
					if err != nil {
						slog.Error("Failed to compare versions", "pkgbase", pi.PkgBase, "error", err)
						return utils.WrapErr(err, "failed to compare package versions")
					}
					if cmp > 0 {
						slog.Debug("Local package is newer", "pkgbase", pi.PkgBase, "local", pi.PkgVer, "remote", rp.MustPKGINFO().PkgVer)
						shoubuild = append(shoubuild, pkg)
					}
				}
				if len(shoubuild) == 0 {
					slog.Info("No packages to build")
					return nil
				}
				t := builder.Target{
					Arch:        "x86_64",
					SignKey:     gpgkey,
					InstallPkgs: srcrepo.Config.InstallPkgs.Files,
				}
				outDir := path.Join(destDir, srcrepo.Config.Name)
				for _, pkg := range shoubuild {
					pkgbase := pkg.MustPKGINFO().PkgBase
					slog.Debug("Starting package build", "pkgbase", pkgbase)
					if err := pkg.Build(&t, outDir); err != nil {
						slog.Error("Package build failed", "pkgbase", pkgbase, "error", err)
						return utils.WrapErr(err, "failed to build package")
					}
					slog.Debug("Package build completed", "pkgbase", pkgbase)
				}
				return nil
			}

			// Normal build
			pkgs, err := pacman_utils.GetCleanPkgBinary(srcrepo.Config.InstallPkgs.Names...)
			if err != nil {
				return utils.WrapErr(err, "failed to get clean package binaries")
			}
			t := builder.Target{
				Arch:        "x86_64",
				SignKey:     gpgkey,
				InstallPkgs: append(srcrepo.Config.InstallPkgs.Files, pkgs...),
			}
			outDir := path.Join(destDir, srcrepo.Config.Name)
			slog.Info("Starting package build", "repo", config.RepoDir, "outdir", outDir, "gpgkey", gpgkey)
			if err := srcrepo.Build(&t, outDir, args...); err != nil {
				return utils.WrapErr(err, "failed to build package")
			}
			slog.Debug("Build completed", "outdir", outDir)
			return nil
		},
	}
	cmd.Flags().StringVarP(&gpgkey, "gpgkey", "g", "", "GPG key for package signing")
	cmd.Flags().BoolVar(&diffMode, "diff", false, "Enable diff build mode (build only new packages)")
	cmd.Flags().StringVarP(&server, "server", "s", "", "Blinky server to compare for diff build")
	return &cmd
}

// Register the package build command as a subcommand
func init() {
	subCmds.Add(buildCmd())
}
