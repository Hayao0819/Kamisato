package cmd

import (
	"log/slog"

	utils "github.com/Hayao0819/Kamisato/internal/utils"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/package"
	"github.com/Hayao0819/Kamisato/pkg/pacman/package/builder"
	"github.com/Hayao0819/Kamisato/pkg/pacman/remote"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	pacman_utils "github.com/Hayao0819/Kamisato/pkg/pacman/utils"
	"github.com/spf13/cobra"
)

func diffBuildCmd() *cobra.Command {
	var server string
	cmd := cobra.Command{
		Use:  "diff-build",
		Args: cobra.NoArgs,
		PreRun: func(cmd *cobra.Command, args []string) {
			cmd.PrintErrln("diff-build is still in development")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			srcrepo, err := repo.GetSrcRepo(config.RepoDir)
			if err != nil {
				return utils.WrapErr(err, "failed to get source repository")
			}

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
					// リモートにパッケージがない
					slog.Warn("package not found in remote repository", "pkgbase", pi.PkgBase)
					shoubuild = append(shoubuild, pkg)
					continue
				}

				// cmp=0 -> src == remote
				// cmp>0 -> local > remote // ローカルの方が新しい
				// cmp<0 -> local < remote // リモートの方が新しい
				cmp, err := pacman_utils.VerCmp(pi.PkgVer, rp.MustPKGINFO().PkgVer)
				if err != nil {
					slog.Error("failed to compare package versions", "pkgbase", pi.PkgBase, "error", err)
					return utils.WrapErr(err, "failed to compare package versions")
				}
				if cmp > 0 {
					// ローカルの方が新しい
					slog.Debug("local package is newer", "pkgbase", pi.PkgBase, "local", pi.PkgVer, "remote", rp.MustPKGINFO().PkgVer)
					shoubuild = append(shoubuild, pkg)
				}
				if cmp < 0 {
					// リモートの方が新しい
					slog.Debug("remote package is newer", "pkgbase", pi.PkgBase, "local", pi.PkgVer, "remote", rp.MustPKGINFO().PkgVer)
					continue
				}
			}

			if len(shoubuild) == 0 {
				slog.Info("no packages to build")
			}

			t := builder.Target{
				Arch:        "x86_64",
				SignKey:     "",
				InstallPkgs: srcrepo.Config.InstallPkgs.Files,
			}

			// Build the packages
			for _, pkg := range shoubuild {
				pkgbase := pkg.MustPKGINFO().PkgBase
				slog.Debug("building package", "pkgbase", pkgbase)
				if err := pkg.Build(&t, config.DestDir); err != nil {
					slog.Error("failed to build package", "pkgbase", pkgbase, "error", err)
					return utils.WrapErr(err, "failed to build package")
				}
				slog.Debug("package built", "pkgbase", pkgbase)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&server, "server", "s", "", "Blinky server to upload to")

	return &cmd
}

func init() {
	subCmds.Add(diffBuildCmd())
}
