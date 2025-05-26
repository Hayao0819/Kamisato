package cmd

import (
	"os"
	"path"
	"path/filepath"

	"log/slog"

	"github.com/Hayao0819/Kamisato/alpm/builder"
	"github.com/Hayao0819/Kamisato/alpm/utils"
	"github.com/Hayao0819/Kamisato/ayaka/gpg"
	"github.com/Hayao0819/Kamisato/ayaka/repo"
	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"
)

func buildCmd() *cobra.Command {
	var gpgkey string
	cmd := cobra.Command{
		Use:   "build",
		Short: "Build packages",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if gpgkey == "" {
				return nil
			}

			slog.Info("gpgkey", "key", gpgkey)
			tmpDir, err := os.MkdirTemp("", "ayaka-")
			if err != nil {
				return errors.Wrap(err, "failed to create temp directory")
			}
			defer os.RemoveAll(tmpDir)

			// Create dummy text file
			dummyFile := path.Join(tmpDir, "dummy.txt")
			if err := os.WriteFile(dummyFile, []byte("dummy"), 0644); err != nil {
				return errors.Wrap(err, "failed to create dummy file")
			}

			// Sign the dummy file
			if err := gpg.SignFile(gpgkey, "", dummyFile); err != nil {
				return errors.Wrap(err, "failed to sign dummy file")
			}

			return nil

		},
		RunE: func(cmd *cobra.Command, args []string) error {
			destDir, err := filepath.Abs(config.DestDir)
			if err != nil {
				return errors.Wrap(err, "failed to get absolute path")
			}

			repoDir, err := filepath.Abs(config.RepoDir)
			if err != nil {
				return errors.Wrap(err, "failed to get absolute path")
			}

			repo, err := repo.GetSrcRepo(repoDir)
			if err != nil {
				return errors.Wrap(err, "failed to get source repository")
			}

			pkgs, err := utils.GetCleanPkgBinary(repo.Config.InstallPkgs.Names...)
			if err != nil {
				return errors.Wrap(err, "failed to get clean package binary")
			}

			t := builder.Target{
				Arch:        "x86_64",
				SignKey:     gpgkey,
				InstallPkgs: append(repo.Config.InstallPkgs.Files, pkgs...),
			}

			// TODO: DestDirにメタデータを作る
			outDir := path.Join(destDir, repo.Config.Name)

			slog.Info("building packages", "repo", config.RepoDir, "outdir", outDir, "gpgkey", gpgkey)
			if err := repo.Build(&t, outDir, args...); err != nil {
				return errors.Wrap(err, "failed to build packages")
			}
			slog.Debug("build done", "outdir", outDir)
			return nil
		},
	}

	cmd.Flags().StringVarP(&gpgkey, "gpgkey", "g", "", "GPG key to sign the package")

	return &cmd
}

func init() {
	subCmds.Add(buildCmd())
}
