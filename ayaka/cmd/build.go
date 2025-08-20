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

// Unified build command: supports normal build and diff-build
func buildCmd() *cobra.Command {
	var gpgkey string
	var diffMode bool
	var server string
	cmd := cobra.Command{
		Use:   "build",
		Short: "Build packages (with --diff for diff-build)",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if gpgkey == "" || diffMode {
				return nil
			}
			slog.Info("gpgkey", "key", gpgkey)
			tmpDir, err := os.MkdirTemp("", "ayaka-")
			if err != nil {
				return utils.WrapErr(err, "failed to create temp directory")
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
				return utils.WrapErr(err, "failed to get absolute path")
			}
			repoDir, err := filepath.Abs(config.RepoDir)
			if err != nil {
				return utils.WrapErr(err, "failed to get absolute path")
			}
			srcrepo, err := repo.GetSrcRepo(repoDir)
			if err != nil {
				return utils.WrapErr(err, "failed to get source repository")
			}

			// diff-build mode
			if diffMode {
				if server == "" {
					server = srcrepo.Config.Server
				}
				slog.Debug("getting diff build", "repo", srcrepo.Config.Name, "server", server)
				rr, err := remote.GetRepoFromURL(server, srcrepo.Config.Name)
				if err != nil {
					return utils.WrapErr(err, "failed to get remote repository")
				}
				shoubuild := []*pkg.Package{}
				for _, pkg := range srcrepo.Pkgs {
					pi := pkg.MustPKGINFO()
					rp := rr.PkgByPkgBase(pi.PkgBase)
					if rp == nil {
						slog.Warn("package not found in remote repository", "pkgbase", pi.PkgBase)
						shoubuild = append(shoubuild, pkg)
						continue
					}
					cmp, err := pacman_utils.VerCmp(pi.PkgVer, rp.MustPKGINFO().PkgVer)
					if err != nil {
						slog.Error("failed to compare package versions", "pkgbase", pi.PkgBase, "error", err)
						return utils.WrapErr(err, "failed to compare package versions")
					}
					if cmp > 0 {
						slog.Debug("local package is newer", "pkgbase", pi.PkgBase, "local", pi.PkgVer, "remote", rp.MustPKGINFO().PkgVer)
						shoubuild = append(shoubuild, pkg)
					}
				}
				if len(shoubuild) == 0 {
					slog.Info("no packages to build")
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
					slog.Debug("building package", "pkgbase", pkgbase)
					if err := pkg.Build(&t, outDir); err != nil {
						slog.Error("failed to build package", "pkgbase", pkgbase, "error", err)
						return utils.WrapErr(err, "failed to build package")
					}
					slog.Debug("package built", "pkgbase", pkgbase)
				}
				return nil
			}

			// normal build
			pkgs, err := pacman_utils.GetCleanPkgBinary(srcrepo.Config.InstallPkgs.Names...)
			if err != nil {
				return utils.WrapErr(err, "failed to get clean package binary")
			}
			t := builder.Target{
				Arch:        "x86_64",
				SignKey:     gpgkey,
				InstallPkgs: append(srcrepo.Config.InstallPkgs.Files, pkgs...),
			}
			outDir := path.Join(destDir, srcrepo.Config.Name)
			slog.Info("building packages", "repo", config.RepoDir, "outdir", outDir, "gpgkey", gpgkey)
			if err := srcrepo.Build(&t, outDir, args...); err != nil {
				return utils.WrapErr(err, "failed to build packages")
			}
			slog.Debug("build done", "outdir", outDir)
			return nil
		},
	}
	cmd.Flags().StringVarP(&gpgkey, "gpgkey", "g", "", "GPG key to sign the package")
	cmd.Flags().BoolVar(&diffMode, "diff", false, "Enable diff-build mode (only build newer packages)")
	cmd.Flags().StringVarP(&server, "server", "s", "", "Blinky server to compare for diff-build")
	return &cmd
}

func init() {
	subCmds.Add(buildCmd())
}
