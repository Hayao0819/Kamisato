package buildcmd

import (
	"context"
	"log/slog"
	"os"
	"path"
	"time"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/ayaka/service/build"
	"github.com/Hayao0819/Kamisato/ayaka/service/source"
	"github.com/Hayao0819/Kamisato/internal/client"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/pacman"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
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
	var publish bool
	var publishURL string
	var publishServer string
	cmd := cobra.Command{
		Use:               "build <srcrepo> [pkgname...]",
		Short:             "Build packages locally (--diff to build only changed packages)",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: shared.CompleteSrcRepoThenPackages,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			repo = args[0]

			if app.From(cmd).GetSrcRepo(repo) == nil {
				return errors.WrapErr(shared.ErrSourceRepoNotFound, repo)
			}

			if !sign {
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

			a := app.From(cmd)
			srcrepo := a.GetSrcRepo(repo)
			if srcrepo == nil {
				return errors.WrapErr(shared.ErrSourceRepoNotFound, repo)
			}
			destDir := srcrepo.DestDir
			if destDir == "" {
				return errors.WrapErr(shared.ErrNoDestDir, repo)
			}
			srcdir := srcrepo.Dir
			if srcdir == "" {
				return errors.WrapErr(shared.ErrNoSourceDir, repo)
			}

			// Regenerate .SRCINFO first so a stale one doesn't build or skip the
			// wrong packages; makepkg may be absent on CI, so warn and carry on.
			if updateSrcinfo {
				srcrepo, err = source.ReloadWithSrcinfo(srcrepo, cmd.ErrOrStderr())
				if err != nil {
					return err
				}
			}

			pkgs, cleanup, err := pacman.GetCleanPkgBinary(srcrepo.Config.InstallPkgs.Names...)
			if err != nil {
				return errors.WrapErr(err, "failed to get clean package binaries")
			}
			defer func() { _ = cleanup.Close() }()

			var signKey string
			if sign {
				signKey = gpgkey
			}
			overrides, err := srcrepo.Config.Build.Overrides(arch)
			if err != nil {
				return errors.WrapErr(err, srcrepo.Config.Name)
			}
			if buildTimeout > 0 {
				overrides.Timeout = buildTimeout
			}
			var host builder.HostConfig
			if a.Config != nil {
				host = a.Config.Builder
			}
			if srcrepo.Config.Build.ArchBuild != "" {
				slog.Warn("Ignoring repository-owned build.archbuild; host executable selection belongs in .ayakarc builder.devtools",
					"archbuild", srcrepo.Config.Build.ArchBuild)
			}
			if executor != "" {
				host.Backend = builder.Kind(executor)
			}
			resolved, err := builder.Resolve(host, overrides, arch)
			if err != nil {
				return errors.WrapErr(err, "failed to resolve build configuration")
			}
			slog.Info("Creating build target", "backend", resolved.Backend, "archbuild", resolved.Devtools.ArchBuild, "installpkgs", pkgs)

			buildTarget := build.Target{
				Config:      resolved,
				Arch:        arch,
				SignKey:     signKey,
				InstallPkgs: append(srcrepo.Config.InstallPkgs.Files, pkgs...),
			}
			if publish {
				upload, err := resolvePublisher(cmd, publishURL, publishServer)
				if err != nil {
					return err
				}
				buildTarget.Publish = func(pkgPaths []string) error {
					return upload(cmd.Context(), srcrepo.Config.Name, pkgPaths...)
				}
			}

			outDir := path.Join(destDir, srcrepo.Config.Name)
			writeDir := path.Join(outDir, buildTarget.Arch)

			if diffMode {
				slog.Info("Starting diff build", "repo", srcdir, "outdir", writeDir, "gpgkey", gpgkey)
				remoteRepo, err := shared.RemoteRepo(diffURL, server, srcrepo, buildTarget.Arch)
				if err != nil {
					return err
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
	cmd.Flags().BoolVar(&publish, "publish", false, "Upload each package to ayato right after it is built (and signed); auth via --publish-url, --publish-server or the saved server login")
	cmd.Flags().StringVar(&publishURL, "publish-url", "", "Publish to this ayato base URL with the API key in "+publishAPIKeyEnv+" (CI); default is the registry default server")
	cmd.Flags().StringVar(&publishServer, "publish-server", "", "Publish to this registered ayato server (default: the registry default); --server keeps its legacy diff meaning")
	cmd.MarkFlagsMutuallyExclusive("publish-url", "publish-server")
	shared.AddRepoServerFlags(&cmd)
	_ = cmd.Flags().MarkDeprecated("server", "use --diff-url to point diff builds at the remote repo db dir")
	cmd.Flags().StringVar(&diffURL, "diff-url", "", "Remote repo db dir for diff builds (.../repo/<repo>/<arch>); overrides repo.json url")
	cmd.Flags().StringVar(&executor, "executor", "", "Local build backend: chroot, container or bwrap (default: builder.backend or chroot)")
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Target architecture for the build")
	cmd.Flags().BoolVar(&updateSrcinfo, "update-srcinfo", true, "Regenerate .SRCINFO from PKGBUILD before building (requires makepkg; skipped when absent)")
	cmd.Flags().DurationVar(&buildTimeout, "timeout", 0, "Build timeout per package (e.g. 3h); 0 uses repo.json build.timeout or the backend default")
	return &cmd
}

// publishAPIKeyEnv carries the CI publish credential; an env var keeps the
// secret out of process argv.
const publishAPIKeyEnv = "AYAKA_PUBLISH_API_KEY" // #nosec G101 -- environment variable name, not a credential

// resolvePublisher picks the upload credential: an explicit --publish-url with
// the X-API-Key from AYAKA_PUBLISH_API_KEY (CI), else the registered server
// named by --publish-server (the registry default when empty). --server keeps
// its legacy diff-URL meaning here, so it never selects the publish server.
func resolvePublisher(cmd *cobra.Command, publishURL, publishServer string) (func(ctx context.Context, repo string, files ...string) error, error) {
	if publishURL != "" {
		key := os.Getenv(publishAPIKeyEnv)
		if key == "" {
			return nil, errors.NewErr("--publish-url requires the API key in " + publishAPIKeyEnv)
		}
		api, err := client.NewPublisher(publishURL, key)
		if err != nil {
			return nil, err
		}
		return api.UploadPackageFiles, nil
	}
	api, err := shared.RepoClientAt(cmd, publishServer)
	if err != nil {
		return nil, err
	}
	return api.UploadPackageFiles, nil
}
