package cmd

import (
	"log/slog"
	"os"
	"path"
	"path/filepath"

	"github.com/Hayao0819/Kamisato/ayaka/gpg"
	"github.com/Hayao0819/Kamisato/internal/utils"
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
			// Get infomation from the configuration
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

			// Create build target
			pkgs, err := pacman_utils.GetCleanPkgBinary(srcrepo.Config.InstallPkgs.Names...)
			if err != nil {
				return utils.WrapErr(err, "failed to get clean package binaries")
			}

			slog.Info("Creating build target", "arch", srcrepo.Config.ArchBuild, "installpkgs", pkgs)

			buildTarget := builder.Target{
				Arch:        "x86_64",
				ArchBuild:   srcrepo.Config.ArchBuild,
				SignKey:     gpgkey,
				InstallPkgs: append(srcrepo.Config.InstallPkgs.Files, pkgs...),
			}

			// If server is not specified, use the one from the configuration
			if server == "" {
				server = srcrepo.Config.Server
			}
			// Normal build
			outDir := path.Join(destDir, srcrepo.Config.Name)

			// Diff build mode
			if diffMode {
				slog.Info("Starting diff build", "repo", config.RepoDir, "outdir", outDir, "gpgkey", gpgkey, "server", server)
				remoteRepo, err := remote.RepoFromURL(server, srcrepo.Config.Name)
				if err != nil {
					return utils.WrapErr(err, "failed to get remote repository")
				}
				if err := srcrepo.DiffBuild(&buildTarget, remoteRepo, destDir, args...); err != nil {
					return utils.WrapErr(err, "failed to perform diff build")
				}
				slog.Debug("Diff build completed", "outdir", outDir)
				return nil
			} else {
				slog.Info("Starting package build", "repo", config.RepoDir, "outdir", outDir, "gpgkey", gpgkey)
				if err := srcrepo.Build(&buildTarget, outDir, args...); err != nil {
					return utils.WrapErr(err, "failed to build package")
				}
				slog.Debug("Build completed", "outdir", outDir)
				return nil
			}
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
