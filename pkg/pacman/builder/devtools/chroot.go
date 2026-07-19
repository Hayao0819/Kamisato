package devtools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder/internal/artifact"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder/internal/errutil"
)

// Backend bypasses the devtools wrapper only when overrides require generated -C/-M configs.
type Backend struct {
	config builder.ResolvedConfig
}

func New(config builder.ResolvedConfig) *Backend {
	return &Backend{config: config}
}

func (b *Backend) Name() string {
	return "chroot"
}

func (b *Backend) Build(ctx context.Context, spec builder.Spec) (*builder.Result, error) {
	if spec.SrcDir == "" {
		return nil, errors.New("chroot backend requires Spec.SrcDir")
	}

	useGenerated := len(b.config.Repositories) > 0 || !b.config.Makepkg.IsZero()

	if !useGenerated && b.config.Devtools.ArchBuild == "" {
		return nil, errors.New("chroot backend requires a devtools wrapper (Config.Devtools.ArchBuild), e.g. extra-x86_64-build")
	}

	if b.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, b.config.Timeout)
		defer cancel()
	}

	// Record existing packages before building. makechrootpkg leaves its output
	// in SrcDir (= CWD), so we only take the diff to avoid mistaking leftovers
	// from a previous build or dependencies placed as InstallPkgs for this
	// build's output.
	baseline, err := artifact.Snapshot(spec.SrcDir)
	if err != nil {
		return nil, err
	}

	var out io.Writer
	if spec.LogWriter != nil {
		out = io.MultiWriter(os.Stdout, spec.LogWriter)
	}

	if useGenerated {
		slog.Info("building package in clean chroot", "dir", spec.SrcDir, "arch", spec.Arch, "repos", len(b.config.Repositories))
		if err := runChrootBuildGenerated(ctx, spec, b.config, out); err != nil {
			return nil, errutil.Wrap(err, "failed to build package in chroot")
		}
	} else {
		slog.Info("building package in clean chroot", "dir", spec.SrcDir, "archbuild", b.config.Devtools.ArchBuild, "arch", spec.Arch)
		if err := runChrootBuild(ctx, spec.SrcDir, b.config.Devtools.ArchBuild, spec.InstallPkgs, out); err != nil {
			return nil, errutil.Wrap(err, "failed to build package in chroot")
		}
	}

	built, err := artifact.Collect(spec.SrcDir, baseline)
	if err != nil {
		return nil, err
	}
	if len(built) == 0 {
		return nil, fmt.Errorf("%w: no package files (*.pkg.tar.*) were produced", builder.ErrBuildFailed)
	}

	packages, err := artifact.MoveToDir(built, spec.SrcDir, spec.OutDir)
	if err != nil {
		return nil, err
	}
	return &builder.Result{Packages: packages}, nil
}

func runChrootBuild(ctx context.Context, dir, archBuild string, installPkgs []string, out io.Writer) error {
	makePkgArgs := []string{"--syncdeps", "--noconfirm", "--log", "--holdver"}
	makeChrootPkgArgs := []string{"-c"}
	for _, pkg := range installPkgs {
		makeChrootPkgArgs = append(makeChrootPkgArgs, "-I", pkg)
	}
	slog.Debug("install packages", "pkgs", installPkgs)

	args := append([]string{archBuild}, "--")
	args = append(args, makeChrootPkgArgs...)
	args = append(args, "--")
	args = append(args, makePkgArgs...)

	build := cmdContext(ctx, dir, out, args...)
	slog.Debug("build command", "cmd", build.String())
	if err := build.Run(); err != nil {
		return errutil.BuildFailure(ctx, err, "devtools wrapper build failed")
	}
	return nil
}

func runChrootBuildGenerated(ctx context.Context, spec builder.Spec, config builder.ResolvedConfig, out io.Writer) error {
	arch := spec.Arch
	if arch == "" {
		arch = "x86_64"
	}
	repoName := "extra"
	if config.Devtools.ArchBuild != "" {
		repoName = repoFromArchBuild(config.Devtools.ArchBuild)
	}

	pacConf, err := renderChrootPacmanConf(repoName, config.Repositories)
	if err != nil {
		return err
	}
	mkConf, err := renderChrootMakepkgConf(arch, config.Makepkg)
	if err != nil {
		return err
	}

	pacTmp, cleanupPac, err := writeTempConf("ayaka-pacman-*.conf", pacConf)
	if err != nil {
		return err
	}
	defer cleanupPac()
	mkTmp, cleanupMk, err := writeTempConf("ayaka-makepkg-*.conf", mkConf)
	if err != nil {
		return err
	}
	defer cleanupMk()

	chrootDir, err := os.MkdirTemp("", "ayaka-chroot-")
	if err != nil {
		return errutil.Wrap(err, "failed to create chroot dir")
	}
	defer func() { _ = os.RemoveAll(chrootDir) }()
	chrootRoot := filepath.Join(chrootDir, "root")

	create := cmdContext(ctx, spec.SrcDir, out, mkarchrootArgs(arch, pacTmp, mkTmp, chrootRoot)...)
	slog.Debug("chroot create command", "cmd", create.String())
	if err := create.Run(); err != nil {
		return errutil.Wrap(err, "failed to create chroot (needs root, the 'devtools' package, and systemd-nspawn)")
	}

	build := cmdContext(ctx, spec.SrcDir, out, makechrootpkgArgs(chrootDir, spec.InstallPkgs)...)
	slog.Debug("chroot build command", "cmd", build.String())
	if err := build.Run(); err != nil {
		return errutil.BuildFailure(ctx, err, "makechrootpkg build failed")
	}
	return nil
}

// writeTempConf writes content to a temp file (world-readable: arch-nspawn copies -M/-C into the chroot and must read it there).
func writeTempConf(pattern, content string) (string, func(), error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", nil, errutil.Wrap(err, "failed to create temp config")
	}
	name := f.Name()
	cleanup := func() { _ = os.Remove(name) }
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		cleanup()
		return "", nil, errutil.Wrap(err, "failed to write temp config")
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, errutil.Wrap(err, "failed to write temp config")
	}
	if err := os.Chmod(name, 0o644); err != nil { //nolint:gosec // config copied into the chroot must be readable there
		cleanup()
		return "", nil, errutil.Wrap(err, "failed to chmod temp config")
	}
	return name, cleanup, nil
}

func cmdContext(ctx context.Context, dir string, out io.Writer, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...) //nolint:gosec // args are the internally-assembled builder command, not user input
	cmd.Dir = dir
	if out == nil {
		out = os.Stdout
	}
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Env = os.Environ()
	return cmd
}
