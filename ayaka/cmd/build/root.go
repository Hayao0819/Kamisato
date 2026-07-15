package buildcmd

import (
	"log/slog"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/build"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/pkg/pacman/alpm"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	pacmansign "github.com/Hayao0819/Kamisato/pkg/pacman/sign"
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
	var diffURL string
	var buildTimeout time.Duration
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
				return errors.WrapErr(shared.ErrInvalidRepoName, repo)
			}

			if !sign || diffMode {
				return nil
			}
			if gpgkey == "" {
				return errors.NewErr("--sign requires --key <gpg-key-id>")
			}
			slog.Info("Verifying GPG key", "key", gpgkey)
			tmpDir, err := os.MkdirTemp("", "ayaka-")
			if err != nil {
				return errors.WrapErr(err, "failed to create temporary directory")
			}
			defer os.RemoveAll(tmpDir)
			dummyFile := path.Join(tmpDir, "dummy.txt")
			if err := os.WriteFile(dummyFile, []byte("dummy"), 0o600); err != nil {
				return errors.WrapErr(err, "failed to create dummy file")
			}
			if err := pacmansign.SignFile(gpgkey, "", dummyFile); err != nil {
				return errors.WrapErr(err, "failed to sign dummy file")
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
				return errors.WrapErr(shared.ErrSourceRepoNotFound, repo)
			}
			destDir := app.GetDestDir(repo)
			if destDir == "" {
				return errors.WrapErr(shared.ErrNoDestDir, repo)
			}
			srcdir := app.GetSrcDir(repo)
			if srcdir == "" {
				return errors.WrapErr(shared.ErrNoSourceDir, repo)
			}

			// Regenerate .SRCINFO first so a stale one doesn't build or skip the
			// wrong packages; makepkg may be absent on CI, so warn and carry on.
			if updateSrcinfo {
				if _, lookErr := exec.LookPath("makepkg"); lookErr != nil {
					slog.Warn("skipping .SRCINFO update: makepkg not found on PATH", "error", lookErr)
				} else {
					srcdirs, err := pacmanrepo.GetSrcDirs(srcdir)
					if err != nil {
						return errors.WrapErr(err, "failed to list source directories")
					}
					for _, d := range srcdirs {
						if err := pacmanrepo.GenerateSrcinfo(d, cmd.ErrOrStderr()); err != nil {
							slog.Warn("failed to update .SRCINFO", "dir", d, "error", err)
						}
					}
					// Reload so the freshly written versions drive the diff and build.
					reloaded, err := pacmanrepo.GetSrcRepo(srcdir, srcrepo.Config)
					if err != nil {
						return errors.WrapErr(err, "failed to reload source repo after .SRCINFO update")
					}
					srcrepo = reloaded
				}
			}

			mk := srcrepo.Config.Build.Makepkg
			if mk.Microarch != "" && !builder.ValidMicroarch(mk.Microarch) {
				return errors.NewErr("build.makepkg.microarch " + mk.Microarch + " is not a known x86-64 feature level (x86_64_v2/v3/v4)")
			}

			pkgs, cleanup, err := alpm.GetCleanPkgBinary(srcrepo.Config.InstallPkgs.Names...)
			if err != nil {
				return errors.WrapErr(err, "failed to get clean package binaries")
			}
			defer func() { _ = cleanup.Close() }()

			slog.Info("Creating build target", "archbuild", srcrepo.Config.Build.ArchBuild, "installpkgs", pkgs)

			var signKey string
			if sign {
				signKey = gpgkey
			}
			if a := srcrepo.Config.Build.Arches; len(a) > 0 && !slices.Contains(a, arch) {
				return errors.NewErr("arch " + arch + " is not in " + srcrepo.Config.Name + " build.arches (" + strings.Join(a, ",") + ")")
			}

			// --timeout wins, else repo.json build.timeout, else the backend default.
			resolvedTimeout := buildTimeout
			if resolvedTimeout == 0 && srcrepo.Config.Build.Timeout != "" {
				d, err := time.ParseDuration(srcrepo.Config.Build.Timeout)
				if err != nil {
					return errors.NewErr("build.timeout " + srcrepo.Config.Build.Timeout + " is not a valid duration: " + err.Error())
				}
				resolvedTimeout = d
			}

			buildTarget := builder.Target{
				Arch:        arch,
				ArchBuild:   srcrepo.Config.Build.ArchBuild,
				SignKey:     signKey,
				InstallPkgs: append(srcrepo.Config.InstallPkgs.Files, pkgs...),
				Repos:       repoSpecsFromConfig(srcrepo.Config.Build.Repos),
				Makepkg: builder.MakepkgSettings{
					Packager:     mk.Packager,
					Microarch:    mk.Microarch,
					CFlagsAppend: mk.CFlagsAppend,
					Options:      mk.Options,
				},
				Image:    strings.ReplaceAll(srcrepo.Config.Build.Image, "$arch", arch),
				Executor: builder.Kind(executor),
				Timeout:  resolvedTimeout,
			}

			outDir := path.Join(destDir, srcrepo.Config.Name)
			writeDir := path.Join(outDir, buildTarget.Arch)

			if diffMode {
				diffServer := resolveDiffServer(diffURL, server, srcrepo.Config.URL, buildTarget.Arch)
				slog.Info("Starting diff build", "repo", srcdir, "outdir", writeDir, "gpgkey", gpgkey, "server", diffServer)
				remoteRepo, err := pacmanrepo.RepoFromURL(diffServer, srcrepo.Config.Name)
				if errors.Is(err, pacmanrepo.ErrRepoNotFound) {
					// A repo/arch with no packages yet has no db; treat as empty and build all.
					slog.Warn("remote repo db not found; building everything", "server", diffServer)
					remoteRepo = &pacmanrepo.RemoteRepo{Name: srcrepo.Config.Name}
				} else if err != nil {
					return errors.WrapErr(err, "failed to get remote repository")
				}
				if err := build.Diff(srcrepo, &buildTarget, remoteRepo, outDir, buildPkgs...); err != nil {
					return errors.WrapErr(err, "failed to perform diff build")
				}
				slog.Debug("Diff build completed", "outdir", writeDir)
				return nil
			}

			slog.Info("Starting package build", "repo", srcdir, "outdir", writeDir, "gpgkey", gpgkey)
			if err := build.Repo(srcrepo, &buildTarget, outDir, buildPkgs...); err != nil {
				return errors.WrapErr(err, "failed to build package")
			}
			slog.Debug("Build completed", "outdir", writeDir)
			return nil
		},
	}
	cmd.Flags().BoolVar(&sign, "sign", false, "Sign built packages with the GPG key specified by --key")
	cmd.Flags().StringVar(&gpgkey, "key", "", "GPG key ID for package signing (requires --sign)")
	cmd.Flags().BoolVar(&diffMode, "diff", false, "Enable diff build mode (build only new packages)")
	shared.AddServerFlag(&cmd)
	_ = cmd.Flags().MarkDeprecated("server", "use --diff-url to point diff builds at the remote repo db dir")
	cmd.Flags().StringVar(&diffURL, "diff-url", "", "Remote repo db dir for diff builds (.../repo/<repo>/<arch>); overrides repo.json url")
	cmd.Flags().StringVar(&executor, "executor", "chroot", "Local build backend: chroot or container")
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Target architecture for the build")
	cmd.Flags().BoolVar(&updateSrcinfo, "update-srcinfo", true, "Regenerate .SRCINFO from PKGBUILD before building (requires makepkg; skipped when absent)")
	cmd.Flags().DurationVar(&buildTimeout, "timeout", 0, "Build timeout per package (e.g. 3h); 0 uses repo.json build.timeout or the backend default")
	return &cmd
}

// repoSpecsFromConfig maps the repo.json build.repos entries to the builder's
// RepoSpec so per-build repositories reach the container/bwrap backends.
func repoSpecsFromConfig(repos []pacmanrepo.BuildRepo) []builder.RepoSpec {
	if len(repos) == 0 {
		return nil
	}
	out := make([]builder.RepoSpec, 0, len(repos))
	for _, r := range repos {
		out = append(out, builder.RepoSpec{Name: r.Name, Server: r.Server, SigLevel: r.SigLevel})
	}
	return out
}

// resolveDiffServer picks the remote repo db dir for a diff build: the explicit
// --diff-url, else the deprecated --server, else the arch-less repo.json url with
// the build arch appended. Empty when none is configured.
func resolveDiffServer(diffURL, server, configURL, arch string) string {
	if diffURL != "" {
		return diffURL
	}
	if server != "" {
		return server
	}
	if configURL != "" {
		return strings.TrimRight(configURL, "/") + "/" + arch
	}
	return ""
}
