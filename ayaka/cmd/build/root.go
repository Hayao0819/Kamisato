package buildcmd

import (
	"log/slog"
	"os"
	"path"
	"slices"

	"github.com/Hayao0819/Kamisato/ayaka/build"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/pacman/alpm"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	"github.com/Hayao0819/Kamisato/pkg/pacman/gpg"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/spf13/cobra"
)

// Cmd builds in a clean local chroot by default; --remote submits to miko (via
// ayato) through the same path as `ayaka miko build`.
func Cmd() *cobra.Command {
	var gpgkey string
	var diffMode bool
	var server string
	var repo string
	var remote bool
	var executor string
	var arch string
	cmd := cobra.Command{
		Use:   "build <repo> [packages...]",
		Short: "Build packages locally (--diff for diff build, --remote to build on miko)",
		Args:  cobra.MinimumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return shared.GetSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
			}

			repoName := args[0]
			sr := shared.GetSrcRepo(repoName)
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

			if !slices.Contains(shared.GetSrcRepoNames(), repo) {
				return utils.WrapErr(shared.ErrInvalidRepoName, repo)
			}

			// In remote mode signing happens on miko, so skip the local key
			// check here.
			if remote {
				return nil
			}

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
			var buildPkgs []string
			if len(args) > 1 {
				buildPkgs = args[1:]
			}

			if remote {
				return shared.RunRemoteBuild(shared.RemoteBuildOpts{
					Repo:   repo,
					Server: server,
					Pkgs:   buildPkgs,
				})
			}

			srcrepo := shared.GetSrcRepo(repo)
			if srcrepo == nil {
				return utils.WrapErr(shared.ErrSourceRepoNotFound, repo)
			}
			destDir := shared.GetDestDir(repo)
			if destDir == "" {
				return utils.WrapErr(shared.ErrNoDestDir, repo)
			}
			srcdir := shared.GetSrcDir(repo)
			if srcdir == "" {
				return utils.WrapErr(shared.ErrNoSourceDir, repo)
			}

			pkgs, cleanup, err := alpm.GetCleanPkgBinary(srcrepo.Config.InstallPkgs.Names...)
			if err != nil {
				return utils.WrapErr(err, "failed to get clean package binaries")
			}
			// The downloaded files are injected during the build, so keep them
			// until it finishes.
			defer func() { _ = cleanup.Close() }()

			slog.Info("Creating build target", "arch", srcrepo.Config.ArchBuild, "installpkgs", pkgs)

			buildTarget := builder.Target{
				Arch:        arch,
				ArchBuild:   srcrepo.Config.ArchBuild,
				SignKey:     gpgkey,
				InstallPkgs: append(srcrepo.Config.InstallPkgs.Files, pkgs...),
				Executor:    builder.Kind(executor),
			}

			if server == "" {
				server = srcrepo.Config.Server
			}
			outDir := path.Join(destDir, srcrepo.Config.Name)
			// Repo and Diff both append the arch subdir; this is where they write.
			writeDir := path.Join(outDir, buildTarget.Arch)

			if diffMode {
				slog.Info("Starting diff build", "repo", srcdir, "outdir", writeDir, "gpgkey", gpgkey, "server", server)
				remoteRepo, err := pacmanrepo.RepoFromURL(server, srcrepo.Config.Name)
				if err != nil {
					return utils.WrapErr(err, "failed to get remote repository")
				}
				if err := build.Diff(srcrepo, &buildTarget, remoteRepo, outDir, buildPkgs...); err != nil {
					return utils.WrapErr(err, "failed to perform diff build")
				}
				slog.Debug("Diff build completed", "outdir", writeDir)
				return nil
			}

			slog.Info("Starting package build", "repo", srcdir, "outdir", writeDir, "gpgkey", gpgkey)
			if err := build.Repo(srcrepo, &buildTarget, outDir, buildPkgs...); err != nil {
				return utils.WrapErr(err, "failed to build package")
			}
			slog.Debug("Build completed", "outdir", writeDir)
			return nil
		},
	}
	cmd.Flags().StringVarP(&gpgkey, "key", "g", "", "GPG key for package signing")
	// --gpgkey is the original spelling, kept as a deprecated alias for --key.
	// Both bind to the same value.
	cmd.Flags().StringVar(&gpgkey, "gpgkey", "", "Deprecated: use --key")
	_ = cmd.Flags().MarkDeprecated("gpgkey", "use --key instead")
	cmd.Flags().BoolVar(&diffMode, "diff", false, "Enable diff build mode (build only new packages)")
	cmd.Flags().StringVarP(&server, "server", "s", "", "ayato server (diff compare, or --remote target)")
	cmd.Flags().BoolVar(&remote, "remote", false, "Build on miko (via ayato) instead of locally")
	cmd.Flags().StringVar(&executor, "executor", "chroot", "Local build backend: chroot or container")
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Target architecture for the build")
	// Remote builds run on miko and have no diff mode; reject the combination
	// instead of silently ignoring --diff.
	cmd.MarkFlagsMutuallyExclusive("remote", "diff")
	return &cmd
}
