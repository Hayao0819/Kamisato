package buildcmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/ayaka/cli"
	buildsetapp "github.com/Hayao0819/Kamisato/ayaka/internal/buildset"
	"github.com/Hayao0819/Kamisato/ayaka/service/build"
	"github.com/Hayao0819/Kamisato/ayaka/service/plan"
	"github.com/Hayao0819/Kamisato/ayaka/service/source"
	"github.com/Hayao0819/Kamisato/internal/api/ayato"
	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
	pacmanhost "github.com/Hayao0819/Kamisato/internal/pacman/host"
	pacmansign "github.com/Hayao0819/Kamisato/internal/pacman/sign"
	pacmansource "github.com/Hayao0819/Kamisato/internal/pacman/source"
)

func Cmd(runtime *app.Runtime) *cobra.Command {
	var input cli.DirectBuildFlags
	var sign bool
	var gpgkey string
	var diffMode bool
	var sourceRepo string
	var executor string
	var updateSrcinfo bool
	var diffURL string
	var publish bool
	var publishURL string
	var publishServer string
	var repoName string
	var output string
	var manifestPath string
	var logDir string
	cmd := cobra.Command{
		Use:               "build [pkgname...]",
		Short:             "Build packages locally",
		ValidArgsFunction: cli.CompleteSrcRepoPackages(runtime, func() string { return sourceRepo }),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if executor != "" {
				input.Backend = executor
			}
			if sourceRepo == "" {
				if err := rejectChangedFlags(cmd, "direct package builds", sourceBuildFlags...); err != nil {
					return err
				}
				if err := input.Validate(args); err != nil {
					return err
				}
				if output == "" || manifestPath == "" {
					return &cmdline.UsageError{Err: fmt.Errorf("--output and --manifest are required for direct package builds")}
				}
				return nil
			}
			if err := rejectChangedFlags(cmd, "--source-repo builds", directBuildFlags...); err != nil {
				return err
			}
			a, err := runtime.App()
			if err != nil {
				return err
			}
			if a.GetSrcRepo(sourceRepo) == nil {
				return errors.WrapErr(cli.ErrSourceRepoNotFound, sourceRepo)
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
			if sourceRepo == "" {
				return runDirectBuild(cmd, args, input, repoName, output, manifestPath, logDir)
			}
			a, err := runtime.App()
			if err != nil {
				return err
			}
			server, err := cmd.Flags().GetString("server")
			if err != nil {
				return err
			}
			buildPkgs := args

			srcrepo := a.GetSrcRepo(sourceRepo)
			if srcrepo == nil {
				return errors.WrapErr(cli.ErrSourceRepoNotFound, sourceRepo)
			}
			destDir := srcrepo.DestDir
			if destDir == "" {
				return errors.WrapErr(cli.ErrNoDestDir, sourceRepo)
			}
			srcdir := srcrepo.Dir
			if srcdir == "" {
				return errors.WrapErr(cli.ErrNoSourceDir, sourceRepo)
			}

			// Regenerate .SRCINFO first so a stale one doesn't build or skip the
			// wrong packages; makepkg may be absent on CI, so warn and carry on.
			if updateSrcinfo {
				srcrepo, err = source.ReloadWithSrcinfo(srcrepo, cmd.ErrOrStderr())
				if err != nil {
					return err
				}
			}

			var signKey string
			if sign {
				signKey = gpgkey
			}
			overrides, err := srcrepo.Config.Build.Overrides(input.Arch)
			if err != nil {
				return errors.WrapErr(err, srcrepo.Config.Name)
			}
			if input.Timeout > 0 {
				overrides.Timeout = input.Timeout
			}
			if input.Image != "" {
				overrides.DockerImage = input.Image
			}
			var host builder.HostConfig
			if a.Config != nil {
				host = a.Config.Builder
			}
			if srcrepo.Config.Build.ArchBuild != "" {
				slog.Warn("Ignoring repository-owned build.archbuild; host executable selection belongs in .ayakarc builder.devtools",
					"archbuild", srcrepo.Config.Build.ArchBuild)
			}
			if input.Backend != "" {
				host.Backend = builder.Kind(input.Backend)
			}
			resolved, err := builder.Resolve(host, overrides, input.Arch)
			if err != nil {
				return errors.WrapErr(err, "failed to resolve build configuration")
			}
			var pkgs, installNames []string
			if resolved.Backend == builder.KindContainer {
				installNames = append([]string(nil), srcrepo.Config.InstallPkgs.Names...)
			} else {
				var cleanup *pacmanhost.CleanPkgBinary
				pkgs, cleanup, err = pacmanhost.GetCleanPkgBinary(srcrepo.Config.InstallPkgs.Names...)
				if err != nil {
					return errors.WrapErr(err, "failed to get clean package binaries")
				}
				defer func() { _ = cleanup.Close() }()
			}
			slog.Info("Creating build target", "backend", resolved.Backend, "archbuild", resolved.Devtools.ArchBuild, "installpkgs", pkgs)

			buildTarget := build.Target{
				Config:       resolved,
				Arch:         input.Arch,
				SignKey:      signKey,
				InstallPkgs:  append(srcrepo.Config.InstallPkgs.Files, pkgs...),
				InstallNames: installNames,
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
				remoteRepo, err := cli.RemoteRepo(diffURL, server, srcrepo, buildTarget.Arch)
				if err != nil {
					return err
				}
				planInput := srcrepo.Pkgs
				if len(buildPkgs) > 0 {
					planInput = pacmansource.SelectPackages(planInput, buildPkgs)
				}
				buildPlan, err := plan.Compute(cmd.Context(), planInput, remoteRepo, buildTarget.Arch, plan.CascadeOff, 0, nil)
				if err != nil {
					return errors.WrapErr(err, "failed to plan diff build")
				}
				if len(buildPlan.Order) == 0 {
					slog.Info("No packages to build")
					return nil
				}
				if err := build.Repo(srcrepo, &buildTarget, outDir, buildPlan.Order...); err != nil {
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
	input.Add(&cmd)
	cmd.Flags().StringVar(&sourceRepo, "source-repo", "", "Configured source repository")
	cmd.Flags().StringVar(&repoName, "repo", "ayaka-local", "Temporary repository name for direct package builds")
	cmd.Flags().StringVar(&output, "output", "", "Completed repository directory for direct package builds")
	cmd.Flags().StringVar(&manifestPath, "manifest", "", "Result manifest path for direct package builds")
	cmd.Flags().StringVar(&logDir, "log-dir", "", "Directory for per-pkgbase logs")
	cmd.Flags().BoolVar(&sign, "sign", false, "Sign built packages with the GPG key specified by --key")
	cmd.Flags().StringVar(&gpgkey, "key", "", "GPG key ID for package signing (requires --sign)")
	cmd.Flags().BoolVar(&diffMode, "diff", false, "Enable diff build mode (build only changed or missing packages)")
	cmd.Flags().BoolVar(&publish, "publish", false, "Upload each package to ayato right after it is built (and signed); auth via --publish-url, --publish-server or the saved server login")
	cmd.Flags().StringVar(&publishURL, "publish-url", "", "Publish to this ayato base URL with the API key in "+publishAPIKeyEnv+" (CI); default is the registry default server")
	cmd.Flags().StringVar(&publishServer, "publish-server", "", "Publish to this registered ayato server (default: the registry default); --server keeps its legacy diff meaning")
	cmd.MarkFlagsMutuallyExclusive("publish-url", "publish-server")
	cli.AddRepoServerFlags(&cmd)
	_ = cmd.Flags().MarkDeprecated("server", "use --diff-url to point diff builds at the remote repo db dir")
	cmd.Flags().StringVar(&diffURL, "diff-url", "", "Remote repo db dir for diff builds (.../repo/<repo>/<arch>); overrides repo.json url")
	cmd.Flags().StringVar(&executor, "executor", "", "Local build backend: chroot, container or bwrap (default: builder.backend or chroot)")
	_ = cmd.Flags().MarkDeprecated("executor", "use --backend")
	cmd.MarkFlagsMutuallyExclusive("executor", "backend")
	cmd.Flags().BoolVar(&updateSrcinfo, "update-srcinfo", true, "Regenerate .SRCINFO from PKGBUILD before building (requires makepkg; skipped when absent)")
	_ = cmd.RegisterFlagCompletionFunc("source-repo", cli.CompleteSrcRepoFlag(runtime))
	return &cmd
}

var sourceBuildFlags = []string{
	"sign", "key", "diff", "publish", "publish-url", "publish-server", "server", "diff-url",
	"update-srcinfo", "token", "username", "password", "ask-pass",
}

var directBuildFlags = []string{
	"local-source", "pacman-conf", "repo", "output", "manifest", "work-dir", "keep-work", "log-dir",
	"makepkg-option",
}

func rejectChangedFlags(command *cobra.Command, mode string, names ...string) error {
	for _, name := range names {
		if command.Flags().Changed(name) {
			return &cmdline.UsageError{Err: fmt.Errorf("--%s cannot be used with %s", name, mode)}
		}
	}
	return nil
}

func runDirectBuild(
	command *cobra.Command,
	packages []string,
	input cli.DirectBuildFlags,
	repoName, output, manifestPath, logDir string,
) error {
	configFile, err := command.Flags().GetString("config")
	if err != nil {
		return err
	}
	application, environment, err := app.NewDirectBuildApplication(input.ApplicationOptions(configFile))
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(command.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	manifest, err := application.Build(ctx, buildsetapp.BuildRequest{
		PlanRequest: input.PlanRequest(packages),
		RepoName:    repoName,
		OutputDir:   output,
		Manifest:    manifestPath,
		LogDir:      logDir,
		Environment: environment,
	})
	if err != nil {
		return err
	}
	if manifest == nil {
		return fmt.Errorf("direct package build completed without a manifest")
	}
	slog.Info("package build completed", "repository", manifest.Repository.Path, "manifest", manifestPath)
	return nil
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
		api, err := ayato.NewPublisher(publishURL, key)
		if err != nil {
			return nil, err
		}
		return api.UploadPackageFiles, nil
	}
	api, err := cli.RepoClientAt(cmd, publishServer)
	if err != nil {
		return nil, err
	}
	return api.UploadPackageFiles, nil
}
