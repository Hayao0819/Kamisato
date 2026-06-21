package cmd

import (
	"log/slog"
	"os"
	"path"
	"slices"

	"github.com/Hayao0819/Kamisato/ayaka/gpg"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/pacman/alpm"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/spf13/cobra"
)

// buildCmd returns the local build command. By default it builds in a clean
// chroot on this machine; --remote instead submits the build to miko (via
// ayato), delegating to the same code path as `ayaka miko build`.
func buildCmd() *cobra.Command {
	var gpgkey string
	var diffMode bool
	var server string
	var repo string
	var remote bool
	cmd := cobra.Command{
		Use:   "build <repo> [packages...]",
		Short: "Build packages locally (--diff for diff build, --remote to build on miko)",
		Args:  cobra.MinimumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			// 1つ目の引数: リポジトリ名補完
			if len(args) == 0 {
				return getSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
			}

			// 2つ目以降の引数: パッケージ名補完
			repoName := args[0]
			sr := getSrcRepo(repoName)
			if sr == nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			// sr.Pkgs から pkgbase と Names() を列挙
			var cands []string
			for _, p := range sr.Pkgs {
				cands = append(cands, p.Base())
				cands = append(cands, p.Names()...)
			}

			return cands, cobra.ShellCompDirectiveNoFileComp
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			repo = args[0]

			// Validate args
			if !slices.Contains(getSrcRepoNames(), repo) {
				return utils.NewErr("invalid repository name: " + repo)
			}

			// In remote mode signing happens on miko, so skip the local key
			// check here.
			if remote {
				return nil
			}

			// Validate gpg signing key
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
			// Optional package list (2nd argument and later)
			var buildPkgs []string
			if len(args) > 1 {
				buildPkgs = args[1:]
			}

			// Remote build: hand off to the miko build path.
			if remote {
				return runRemoteBuild(remoteBuildOpts{
					repo:   repo,
					server: server,
					gpgkey: gpgkey,
					pkgs:   buildPkgs,
				})
			}

			srcrepo := getSrcRepo(repo)
			if srcrepo == nil {
				return utils.NewErr("failed to get source repository")
			}
			destDir := getDestDir(repo)
			if destDir == "" {
				return utils.NewErr("failed to get destination directory")
			}
			srcdir := getSrcDir(repo)
			if srcdir == "" {
				return utils.NewErr("failed to get source directory")
			}

			// Create build target
			pkgs, err := alpm.GetCleanPkgBinary(srcrepo.Config.InstallPkgs.Names...)
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
				slog.Info("Starting diff build", "repo", srcdir, "outdir", outDir, "gpgkey", gpgkey, "server", server)
				remoteRepo, err := pacmanrepo.RepoFromURL(server, srcrepo.Config.Name)
				if err != nil {
					return utils.WrapErr(err, "failed to get remote repository")
				}
				if err := srcrepo.DiffBuild(&buildTarget, remoteRepo, destDir, buildPkgs...); err != nil {
					return utils.WrapErr(err, "failed to perform diff build")
				}
				slog.Debug("Diff build completed", "outdir", outDir)
				return nil
			}

			slog.Info("Starting package build", "repo", srcdir, "outdir", outDir, "gpgkey", gpgkey)
			if err := srcrepo.Build(&buildTarget, outDir, buildPkgs...); err != nil {
				return utils.WrapErr(err, "failed to build package")
			}
			slog.Debug("Build completed", "outdir", outDir)
			return nil
		},
	}
	cmd.Flags().StringVarP(&gpgkey, "key", "g", "", "GPG key for package signing")
	// --gpgkey is the original spelling, kept as a deprecated alias since the
	// flag was unified with `miko build --key`. Both bind to the same value.
	cmd.Flags().StringVar(&gpgkey, "gpgkey", "", "Deprecated: use --key")
	_ = cmd.Flags().MarkDeprecated("gpgkey", "use --key instead")
	cmd.Flags().BoolVar(&diffMode, "diff", false, "Enable diff build mode (build only new packages)")
	cmd.Flags().StringVarP(&server, "server", "s", "", "ayato server (diff compare, or --remote target)")
	cmd.Flags().BoolVar(&remote, "remote", false, "Build on miko (via ayato) instead of locally")
	// Remote builds run on miko and have no diff mode; reject the combination
	// instead of silently ignoring --diff.
	cmd.MarkFlagsMutuallyExclusive("remote", "diff")
	return &cmd
}

// Register the package build command as a subcommand
func init() {
	subCmds.Add(buildCmd())
}
