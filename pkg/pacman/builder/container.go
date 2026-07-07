package builder

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
)

// buildScript is the in-container entrypoint; __EXTRA_REPOS__ and __INSTALL__ are substituted per build.
//
//go:embed buildscript.sh
var buildScript string

// makepkgOverrideConf is the static base of /build/makepkg.override.conf;
// the entrypoint appends dynamic per-build settings so it's a real file rather than a heredoc.
//
//go:embed makepkg.override.conf
var makepkgOverrideConf string

// containerBackend builds packages in a fresh throwaway container (makecontainerpkg-style clean room).
// Cross-arch requires qemu-user-static registered with binfmt_misc using the "F" (fix_binary) flag.
type containerBackend struct {
	image          string
	timeout        time.Duration
	host           string
	pacmanCacheDir string
	ccacheDir      string
	extraRepos     []RepoSpec
}

func newContainerBackend(opts Options) Backend {
	img := opts.Image
	if img == "" {
		img = defaultContainerImage
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultBuildTimeout
	}
	return &containerBackend{
		image:          img,
		timeout:        timeout,
		host:           opts.DockerHost,
		pacmanCacheDir: opts.PacmanCacheDir,
		ccacheDir:      opts.CcacheDir,
		extraRepos:     opts.ExtraRepos,
	}
}

func (b *containerBackend) Name() string { return "container" }

func (b *containerBackend) Build(ctx context.Context, spec Spec) (*Result, error) {
	if spec.SrcDir == "" {
		return nil, errors.New("container backend requires Spec.SrcDir")
	}
	slog.Info("starting container build", "arch", spec.Arch, "image", b.image)

	platform, err := archToPlatform(spec.Arch)
	if err != nil {
		return nil, wrapErr(err, "failed to resolve platform")
	}
	platformStr := platformString(platform)

	absSrc, err := filepath.Abs(spec.SrcDir)
	if err != nil {
		return nil, wrapErr(err, "failed to resolve src dir")
	}
	outDir := spec.OutDir
	if outDir == "" {
		outDir = spec.SrcDir
	}
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return nil, wrapErr(err, "failed to resolve out dir")
	}
	if err := os.MkdirAll(absOut, 0o755); err != nil { //nolint:gosec // build output dir, read by the build user and downstream consumers
		return nil, wrapErr(err, "failed to create out dir")
	}

	// Snapshot baseline so a build into a non-empty OutDir only reports freshly produced artifacts.
	baseline, err := snapshotPackages(absOut)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()

	cli, err := newDockerClient(b.host)
	if err != nil {
		return nil, wrapErr(err, "failed to create docker client")
	}
	defer cli.Close()

	slog.Info("pulling container image", "image", b.image, "platform", platformStr)
	reader, err := cli.ImagePull(ctx, b.image, image.PullOptions{Platform: platformStr})
	if err != nil {
		return nil, wrapErr(err, "failed to pull image")
	}
	if err := drainPullStream(reader); err != nil {
		return nil, wrapErr(err, "failed to pull image")
	}

	// Shell-quote install paths so a hostile filename can't inject into `sh -c`.
	installMounts := make([]mount.Mount, 0, len(spec.InstallPkgs))
	var installCmd strings.Builder
	for i, pkg := range spec.InstallPkgs {
		absPkg, err := filepath.Abs(pkg)
		if err != nil {
			return nil, wrapErr(err, "failed to resolve install package path")
		}
		target := fmt.Sprintf("/build/install/%d/%s", i, filepath.Base(pkg))
		installMounts = append(installMounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   absPkg,
			Target:   target,
			ReadOnly: true,
		})
		fmt.Fprintf(&installCmd, "pacman -U --noconfirm %s\n", shellQuote(target))
	}

	// ExtraRepos (server-config) precede Spec.Repos (repo.json) in pacman.conf.
	effectiveRepos := append(append([]RepoSpec{}, b.extraRepos...), spec.Repos...)

	script := substituteBuildPlaceholders(buildScript, extraReposScript(effectiveRepos), strings.TrimRight(installCmd.String(), "\n"))

	// Stage per-build makepkg settings as a host temp file for bind-mounting into the container.
	overridePath, cleanupOverride, err := stageOverrideConf(spec.Makepkg)
	if err != nil {
		return nil, err
	}
	defer cleanupOverride()

	containerConfig := &container.Config{
		Image:      b.image,
		Cmd:        []string{"sh", "-c", script},
		Env:        []string{"TARGET_CARCH=" + spec.Arch},
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
			Source: absOut,
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
		return nil, wrapErr(err, "failed to create container")
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
		return nil, wrapErr(err, "failed to start container")
	}

	// Stream container output live via stdcopy (demuxes the multiplexed log); capture feeds error messages on failure.
	capture := &syncBuffer{}
	var logDst io.Writer = capture
	if spec.LogWriter != nil {
		logDst = io.MultiWriter(capture, spec.LogWriter)
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
			return nil, fmt.Errorf("%w with exit code %d:\n%s", ErrBuildFailed, status.StatusCode, capture.String())
		}
	}

	pkgs, err := collectNewPackages(absOut, baseline)
	if err != nil {
		return nil, wrapErr(err, "failed to collect built packages")
	}
	slog.Info("container build completed", "packages", len(pkgs))
	return &Result{Packages: pkgs}, nil
}

// stageOverrideConf writes the embedded makepkg override base plus per-build settings to a host temp file
// for bind-mounting into the container; errors on unknown microarch tier.
func stageOverrideConf(mk MakepkgSettings) (string, func(), error) {
	overrides, err := makepkgOverrideLines(mk)
	if err != nil {
		return "", nil, err
	}
	f, err := os.CreateTemp("", "makepkg-override-*.conf")
	if err != nil {
		return "", nil, wrapErr(err, "failed to stage makepkg override")
	}
	if _, err := f.WriteString(makepkgOverrideConf + overrides); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", nil, wrapErr(err, "failed to write makepkg override")
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", nil, wrapErr(err, "failed to write makepkg override")
	}
	return f.Name(), func() { _ = os.Remove(f.Name()) }, nil
}

// cacheMounts returns the cache bind-mounts, creating host dirs if missing.
func (b *containerBackend) cacheMounts() ([]mount.Mount, error) {
	var mounts []mount.Mount
	add := func(hostDir, target string) error {
		if hostDir == "" {
			return nil
		}
		abs, err := filepath.Abs(hostDir)
		if err != nil {
			return wrapErr(err, "failed to resolve cache dir")
		}
		if err := os.MkdirAll(abs, 0o755); err != nil { //nolint:gosec // cache dir shared with the build container
			return wrapErr(err, "failed to create cache dir")
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
