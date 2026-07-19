package docker

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder/internal/artifact"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder/internal/buildenv"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder/internal/errutil"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder/internal/shellutil"
)

//go:embed buildscript.sh
var buildScript string

// Backend builds packages in a fresh throwaway container.
// Cross-arch requires qemu-user-static registered with binfmt_misc using the "F" (fix_binary) flag.
type Backend struct {
	image          string
	timeout        time.Duration
	host           string
	pacmanCacheDir string
	ccacheDir      string
	extraRepos     []builder.PacmanRepository
	makepkg        builder.MakepkgConfig
}

func New(config builder.ResolvedConfig) *Backend {
	img := config.Docker.Image
	if img == "" {
		img = defaultContainerImage
	}
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = defaultBuildTimeout
	}
	return &Backend{
		image:          img,
		timeout:        timeout,
		host:           config.Docker.Host,
		pacmanCacheDir: config.Docker.PacmanCacheDir,
		ccacheDir:      config.Docker.CcacheDir,
		extraRepos:     config.Repositories,
		makepkg:        config.Makepkg,
	}
}

func (b *Backend) Name() string { return "container" }

func (b *Backend) Build(ctx context.Context, spec builder.Spec) (*builder.Result, error) {
	if spec.SrcDir == "" {
		return nil, errors.New("container backend requires Spec.SrcDir")
	}
	slog.Info("starting container build", "arch", spec.Arch, "image", b.image)

	platform, err := archToPlatform(spec.Arch)
	if err != nil {
		return nil, errutil.Wrap(err, "failed to resolve platform")
	}
	platformStr := platformString(platform)

	absSrc, err := filepath.Abs(spec.SrcDir)
	if err != nil {
		return nil, errutil.Wrap(err, "failed to resolve src dir")
	}
	outDir := spec.OutDir
	if outDir == "" {
		outDir = spec.SrcDir
	}
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return nil, errutil.Wrap(err, "failed to resolve out dir")
	}
	if err := os.MkdirAll(absOut, 0o755); err != nil { //nolint:gosec // build output dir, read by the build user and downstream consumers
		return nil, errutil.Wrap(err, "failed to create out dir")
	}
	stagingOut, err := os.MkdirTemp("", "kamisato-docker-out-*")
	if err != nil {
		return nil, errutil.Wrap(err, "failed to create build output staging dir")
	}
	defer func() { _ = os.RemoveAll(stagingOut) }()
	if err := os.Chmod(stagingOut, 0o755); err != nil { //nolint:gosec // the build container must be able to traverse this host mount
		return nil, errutil.Wrap(err, "failed to prepare build output staging dir")
	}

	ctx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()

	cli, err := newDockerClient(b.host)
	if err != nil {
		return nil, errutil.Wrap(err, "failed to create docker client")
	}
	defer cli.Close()

	slog.Info("pulling container image", "image", b.image, "platform", platformStr)
	reader, err := cli.ImagePull(ctx, b.image, image.PullOptions{Platform: platformStr})
	if err != nil {
		return nil, errutil.Wrap(err, "failed to pull image")
	}
	if err := drainPullStream(reader); err != nil {
		return nil, errutil.Wrap(err, "failed to pull image")
	}

	// Shell-quote install paths so a hostile filename can't inject into `sh -c`.
	installMounts := make([]mount.Mount, 0, len(spec.InstallPkgs))
	var installCmd strings.Builder
	for i, pkg := range spec.InstallPkgs {
		absPkg, err := filepath.Abs(pkg)
		if err != nil {
			return nil, errutil.Wrap(err, "failed to resolve install package path")
		}
		target := fmt.Sprintf("/build/install/%d/%s", i, filepath.Base(pkg))
		installMounts = append(installMounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   absPkg,
			Target:   target,
			ReadOnly: true,
		})
		fmt.Fprintf(&installCmd, "pacman -U --noconfirm %s\n", shellutil.Quote(target))
	}

	reposScript, err := buildenv.ExtraReposScript(b.extraRepos)
	if err != nil {
		return nil, errutil.Wrap(err, "invalid build repository configuration")
	}
	script := buildenv.SubstituteBuildPlaceholders(buildScript, reposScript, strings.TrimRight(installCmd.String(), "\n"))

	overridePath, cleanupOverride, err := buildenv.StageOverrideConf(b.makepkg)
	if err != nil {
		return nil, err
	}
	defer cleanupOverride()

	containerConfig := &container.Config{
		Image:      b.image,
		Cmd:        []string{"sh", "-c", script},
		Env:        []string{"TARGET_CARCH=" + spec.Arch, "TARGET_CHOST=" + archToCHOST(spec.Arch)},
		WorkingDir: "/build",
		Tty:        false,
		User:       "root",
	}

	// src is read-only; the script copies it to a writable work dir so the caller's source tree is never mutated.
	mounts := []mount.Mount{
		{
			Type:     mount.TypeBind,
			Source:   absSrc,
			Target:   "/build/src",
			ReadOnly: true,
		},
		{
			Type:   mount.TypeBind,
			Source: stagingOut,
			Target: "/build/out",
		},
		{
			Type:     mount.TypeBind,
			Source:   overridePath,
			Target:   "/build/staging/makepkg.override.conf",
			ReadOnly: true,
		},
	}
	mounts = append(mounts, installMounts...)

	cacheMounts, err := b.cacheMounts()
	if err != nil {
		return nil, err
	}
	mounts = append(mounts, cacheMounts...)

	hostConfig := &container.HostConfig{
		AutoRemove: false,
		Mounts:     mounts,
	}

	resp, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, platform, "")
	if err != nil {
		return nil, errutil.Wrap(err, "failed to create container")
	}
	containerID := resp.ID
	defer func() {
		// Use a detached context: ctx may already be cancelled/timed out.
		rmCtx, rmCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer rmCancel()
		stopTimeout := 10
		_ = cli.ContainerStop(rmCtx, containerID, container.StopOptions{Timeout: &stopTimeout})
		_ = cli.ContainerRemove(rmCtx, containerID, container.RemoveOptions{Force: true})
	}()

	if err := cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return nil, errutil.Wrap(err, "failed to start container")
	}

	// Docker multiplexes stdout and stderr on this stream.
	capture := &syncBuffer{}
	var logDst io.Writer = io.MultiWriter(os.Stdout, capture)
	if spec.LogWriter != nil {
		logDst = io.MultiWriter(os.Stdout, capture, spec.LogWriter)
	}
	logDone := make(chan struct{})
	logs, logErr := cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true, ShowStderr: true, Follow: true,
	})
	if logErr != nil {
		close(logDone)
	} else {
		go func() {
			defer close(logDone)
			defer logs.Close()
			_, _ = stdcopy.StdCopy(logDst, logDst, logs)
		}()
	}

	statusCh, errCh := cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("build cancelled or timed out: %w\n%s", ctx.Err(), capture.String())
	case err := <-errCh:
		if err != nil {
			return nil, fmt.Errorf("error waiting for container: %w\n%s", err, capture.String())
		}
	case status := <-statusCh:
		// Let the follow stream flush any trailing output before reading capture.
		select {
		case <-logDone:
		case <-time.After(2 * time.Second):
		}
		if status.StatusCode != 0 {
			return nil, fmt.Errorf("%w with exit code %d:\n%s", builder.ErrBuildFailed, status.StatusCode, capture.String())
		}
	}

	packages, err := collectStagedPackages(stagingOut, absOut)
	if err != nil {
		return nil, err
	}
	slog.Info("container build completed", "packages", len(packages))
	return &builder.Result{Packages: packages}, nil
}

func collectStagedPackages(stagingOut, outDir string) ([]string, error) {
	packages, err := artifact.Collect(stagingOut, nil)
	if err != nil {
		return nil, errutil.Wrap(err, "failed to collect built packages")
	}
	if len(packages) == 0 {
		return nil, fmt.Errorf("%w: no package files (*.pkg.tar.*) were produced", builder.ErrBuildFailed)
	}
	moved, err := artifact.MoveToDir(packages, stagingOut, outDir)
	if err != nil {
		return nil, errutil.Wrap(err, "failed to move built packages")
	}
	return moved, nil
}

func (b *Backend) cacheMounts() ([]mount.Mount, error) {
	var mounts []mount.Mount
	add := func(hostDir, target string) error {
		if hostDir == "" {
			return nil
		}
		abs, err := filepath.Abs(hostDir)
		if err != nil {
			return errutil.Wrap(err, "failed to resolve cache dir")
		}
		if err := os.MkdirAll(abs, 0o755); err != nil { //nolint:gosec // cache dir shared with the build container
			return errutil.Wrap(err, "failed to create cache dir")
		}
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: abs,
			Target: target,
		})
		return nil
	}
	if err := add(b.pacmanCacheDir, "/var/cache/pacman/pkg"); err != nil {
		return nil, err
	}
	if err := add(b.ccacheDir, "/build/ccache"); err != nil {
		return nil, err
	}
	return mounts, nil
}
