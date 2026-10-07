package docker

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
	"github.com/Hayao0819/Kamisato/internal/pacman/builder/internal/artifact"
	"github.com/Hayao0819/Kamisato/internal/pacman/builder/internal/buildenv"
	"github.com/Hayao0819/Kamisato/internal/pacman/builder/internal/failure"
	"github.com/Hayao0819/Kamisato/internal/pacman/builder/internal/shell"
)

//go:embed buildscript.sh
var buildScript string

const metadataScript = `set -eu
__PACMAN_CONFIG__
pacman -Sy --noconfirm --needed base-devel
useradd -m builduser 2>/dev/null || true
cp /build/staging/makepkg.override.conf /build/makepkg.override.conf
printf 'CARCH="%s"\nCHOST="%s"\n' "$TARGET_CARCH" "$TARGET_CHOST" >> /build/makepkg.override.conf
cp -r /build/src /build/work
chown -R builduser:builduser /build/work /build/makepkg.override.conf
chmod 0777 /build/out
runuser -u builduser -- sh -c 'cd /build/work && makepkg --config /build/makepkg.override.conf --printsrcinfo > /build/out/.SRCINFO'
`

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
	digestMu       sync.RWMutex
	digest         string
	pinnedImage    string
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

func (b *Backend) BuildEnvironment() builder.BuildEnvironment {
	b.digestMu.RLock()
	defer b.digestMu.RUnlock()
	return builder.BuildEnvironment{Image: b.image, Digest: b.digest}
}

func (b *Backend) Build(ctx context.Context, spec builder.Spec) (*builder.Result, error) {
	if spec.SrcDir == "" {
		return nil, errors.New("container backend requires Spec.SrcDir")
	}
	slog.Info("starting container build", "arch", spec.Arch, "image", b.image)

	platform, err := archToPlatform(spec.Arch)
	if err != nil {
		return nil, failure.Wrap(err, "failed to resolve platform")
	}
	platformStr := platformString(platform)

	absSrc, err := filepath.Abs(spec.SrcDir)
	if err != nil {
		return nil, failure.Wrap(err, "failed to resolve src dir")
	}
	outDir := spec.OutDir
	if outDir == "" {
		outDir = spec.SrcDir
	}
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return nil, failure.Wrap(err, "failed to resolve out dir")
	}
	if err := os.MkdirAll(absOut, 0o755); err != nil { //nolint:gosec // build output dir, read by the build user and downstream consumers
		return nil, failure.Wrap(err, "failed to create out dir")
	}
	stagingOut, err := os.MkdirTemp(filepath.Dir(absOut), ".kamisato-docker-out-*")
	if err != nil {
		return nil, failure.Wrap(err, "failed to create build output staging dir")
	}
	defer func() { _ = os.RemoveAll(stagingOut) }()
	if err := os.Chmod(stagingOut, 0o755); err != nil { //nolint:gosec // the build container must be able to traverse this host mount
		return nil, failure.Wrap(err, "failed to prepare build output staging dir")
	}

	ctx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()

	cli, err := newDockerClient(b.host)
	if err != nil {
		return nil, failure.Wrap(err, "failed to create docker client")
	}
	defer cli.Close()

	imageRef := b.currentImageReference()
	slog.Info("pulling container image", "image", imageRef, "platform", platformStr)
	reader, err := cli.ImagePull(ctx, imageRef, image.PullOptions{Platform: platformStr})
	if err != nil {
		return nil, failure.Wrap(err, "failed to pull image")
	}
	if err := drainPullStream(reader); err != nil {
		return nil, failure.Wrap(err, "failed to pull image")
	}
	b.recordImageDigest(ctx, cli, imageRef)
	imageRef = b.currentImageReference()

	// Shell-quote install paths so a hostile filename can't inject into `sh -c`.
	installMounts := make([]mount.Mount, 0, len(spec.InstallPkgs))
	installTargets := make([]string, 0, len(spec.InstallPkgs))
	for i, pkg := range spec.InstallPkgs {
		absPkg, err := filepath.Abs(pkg)
		if err != nil {
			return nil, failure.Wrap(err, "failed to resolve install package path")
		}
		target := fmt.Sprintf("/build/install/%d/%s", i, filepath.Base(pkg))
		installMounts = append(installMounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   absPkg,
			Target:   target,
			ReadOnly: true,
		})
		installTargets = append(installTargets, shell.Quote(target))
	}
	installCommand, err := installNamesCommand(spec.InstallNames)
	if err != nil {
		return nil, err
	}
	if len(installTargets) > 0 {
		installCommand += "pacman -U --asdeps --noconfirm -- " + strings.Join(installTargets, " ")
	}

	repositories := b.extraRepos
	if spec.PacmanConf != "" {
		repositories = nil
	}
	reposScript, err := buildenv.ExtraReposScript(repositories)
	if err != nil {
		return nil, failure.Wrap(err, "invalid build repository configuration")
	}
	script := buildenv.SubstituteBuildPlaceholders(buildScript, reposScript, installCommand)
	script = strings.ReplaceAll(script, "__MAKEPKG_ARGS__", strings.Join(spec.MakepkgArgs(), " "))
	script = strings.ReplaceAll(script, "__PACMAN_CONFIG__", pacmanConfigScript(spec.PacmanConf))

	overridePath, cleanupOverride, err := buildenv.StageOverrideConfIn(filepath.Dir(absOut), b.makepkg)
	if err != nil {
		return nil, err
	}
	defer cleanupOverride()

	containerConfig := &container.Config{
		Image:      imageRef,
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
	if spec.PacmanConf != "" {
		absConfig, err := filepath.Abs(spec.PacmanConf)
		if err != nil {
			return nil, failure.Wrap(err, "failed to resolve pacman config")
		}
		mounts = append(mounts, mount.Mount{Type: mount.TypeBind, Source: absConfig, Target: "/build/staging/pacman.conf", ReadOnly: true})
	}
	mounts = append(mounts, installMounts...)
	repositoryMounts, err := localRepositoryMounts(spec.LocalRepositoryDirs)
	if err != nil {
		return nil, failure.Wrap(err, "failed to prepare local repository mounts")
	}
	mounts = append(mounts, repositoryMounts...)

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
		return nil, failure.Wrap(err, "failed to create container")
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
		return nil, failure.Wrap(err, "failed to start container")
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
		if logs != nil {
			_ = logs.Close()
		}
		close(logDone)
	} else {
		go func() {
			defer close(logDone)
			defer logs.Close()
			_, _ = stdcopy.StdCopy(logDst, logDst, logs)
		}()
		defer func() {
			_ = logs.Close()
			<-logDone
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
		if logs != nil {
			awaitContainerLogs(logDone, logs, 2*time.Second)
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

func (b *Backend) GenerateSRCINFO(ctx context.Context, spec builder.Spec) ([]byte, error) {
	if spec.SrcDir == "" {
		return nil, errors.New("container metadata generation requires Spec.SrcDir")
	}
	platform, err := archToPlatform(spec.Arch)
	if err != nil {
		return nil, failure.Wrap(err, "failed to resolve platform")
	}
	absSrc, err := filepath.Abs(spec.SrcDir)
	if err != nil {
		return nil, failure.Wrap(err, "failed to resolve src dir")
	}
	sharedParent := filepath.Dir(absSrc)
	stagingOut, err := os.MkdirTemp(sharedParent, ".kamisato-srcinfo-out-*")
	if err != nil {
		return nil, failure.Wrap(err, "failed to create metadata output dir")
	}
	defer func() { _ = os.RemoveAll(stagingOut) }()
	if err := os.Chmod(stagingOut, 0o755); err != nil { //nolint:gosec // container root must traverse the bind mount
		return nil, failure.Wrap(err, "failed to prepare metadata output dir")
	}

	ctx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()
	cli, err := newDockerClient(b.host)
	if err != nil {
		return nil, failure.Wrap(err, "failed to create docker client")
	}
	defer cli.Close()
	imageRef := b.currentImageReference()
	reader, err := cli.ImagePull(ctx, imageRef, image.PullOptions{Platform: platformString(platform)})
	if err != nil {
		return nil, failure.Wrap(err, "failed to pull image")
	}
	if err := drainPullStream(reader); err != nil {
		return nil, failure.Wrap(err, "failed to pull image")
	}
	b.recordImageDigest(ctx, cli, imageRef)
	imageRef = b.currentImageReference()

	overridePath, cleanupOverride, err := buildenv.StageOverrideConfIn(sharedParent, b.makepkg)
	if err != nil {
		return nil, err
	}
	defer cleanupOverride()
	mounts := []mount.Mount{
		{Type: mount.TypeBind, Source: absSrc, Target: "/build/src", ReadOnly: true},
		{Type: mount.TypeBind, Source: stagingOut, Target: "/build/out"},
		{Type: mount.TypeBind, Source: overridePath, Target: "/build/staging/makepkg.override.conf", ReadOnly: true},
	}
	if spec.PacmanConf != "" {
		absConfig, err := filepath.Abs(spec.PacmanConf)
		if err != nil {
			return nil, failure.Wrap(err, "failed to resolve pacman config")
		}
		mounts = append(mounts, mount.Mount{Type: mount.TypeBind, Source: absConfig, Target: "/build/staging/pacman.conf", ReadOnly: true})
	}
	repositoryMounts, err := localRepositoryMounts(spec.LocalRepositoryDirs)
	if err != nil {
		return nil, failure.Wrap(err, "failed to prepare local repository mounts")
	}
	mounts = append(mounts, repositoryMounts...)
	script := strings.ReplaceAll(metadataScript, "__PACMAN_CONFIG__", pacmanConfigScript(spec.PacmanConf))
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: imageRef,
		Cmd:   []string{"sh", "-c", script},
		Env:   []string{"TARGET_CARCH=" + spec.Arch, "TARGET_CHOST=" + archToCHOST(spec.Arch)},
		User:  "root",
	}, &container.HostConfig{AutoRemove: false, Mounts: mounts}, nil, platform, "")
	if err != nil {
		return nil, failure.Wrap(err, "failed to create metadata container")
	}
	containerID := resp.ID
	defer func() {
		rmCtx, rmCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer rmCancel()
		_ = cli.ContainerRemove(rmCtx, containerID, container.RemoveOptions{Force: true})
	}()
	if err := cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return nil, failure.Wrap(err, "failed to start metadata container")
	}
	statusCh, errCh := cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("metadata generation cancelled or timed out: %w", ctx.Err())
	case err := <-errCh:
		if err != nil {
			return nil, fmt.Errorf("error waiting for metadata container: %w", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			logs, _ := cli.ContainerLogs(context.Background(), containerID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
			var captured bytes.Buffer
			if logs != nil {
				_, _ = stdcopy.StdCopy(&captured, &captured, logs)
				_ = logs.Close()
			}
			return nil, fmt.Errorf("metadata generation failed with exit code %d: %s", status.StatusCode, captured.String())
		}
	}
	data, err := os.ReadFile(filepath.Join(stagingOut, ".SRCINFO"))
	if err != nil {
		return nil, failure.Wrap(err, "failed to read generated .SRCINFO")
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("generated .SRCINFO is empty")
	}
	return data, nil
}

func pacmanConfigScript(path string) string {
	if path == "" {
		return ""
	}
	return "install -m 0644 /build/staging/pacman.conf /etc/pacman.conf"
}

func (b *Backend) currentImageReference() string {
	b.digestMu.RLock()
	defer b.digestMu.RUnlock()
	if b.pinnedImage != "" {
		return b.pinnedImage
	}
	return b.image
}

func (b *Backend) recordImageDigest(ctx context.Context, cli interface {
	ImageInspect(context.Context, string, ...client.ImageInspectOption) (image.InspectResponse, error)
}, imageRef string) {
	inspected, err := cli.ImageInspect(ctx, imageRef)
	if err != nil {
		return
	}
	pinned := matchingRepoDigest(imageRef, inspected.RepoDigests)
	if pinned == "" && strings.Contains(imageRef, "@") {
		pinned = imageRef
	}
	_, digest, found := strings.Cut(pinned, "@")
	if !found {
		return
	}
	b.digestMu.Lock()
	if b.pinnedImage == "" {
		b.pinnedImage = pinned
		b.digest = digest
	}
	b.digestMu.Unlock()
}

func matchingRepoDigest(imageRef string, repoDigests []string) string {
	repository := imageRepository(imageRef)
	digests := append([]string(nil), repoDigests...)
	slices.Sort(digests)
	for _, digest := range digests {
		name, _, found := strings.Cut(digest, "@")
		if found && imageRepository(name) == repository {
			return digest
		}
	}
	if len(digests) == 1 {
		return digests[0]
	}
	return ""
}

func imageRepository(reference string) string {
	repository, _, _ := strings.Cut(reference, "@")
	lastSlash := strings.LastIndexByte(repository, '/')
	if tag := strings.LastIndexByte(repository, ':'); tag > lastSlash {
		repository = repository[:tag]
	}
	return repository
}

func awaitContainerLogs(done <-chan struct{}, logs io.Closer, timeout time.Duration) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return
	case <-timer.C:
	}
	_ = logs.Close()
	<-done
}

func collectStagedPackages(stagingOut, outDir string) ([]string, error) {
	return artifact.CollectToDir(stagingOut, nil, outDir)
}

func localRepositoryMounts(directories []string) ([]mount.Mount, error) {
	paths := make([]string, 0, len(directories))
	for _, directory := range directories {
		absolute, err := filepath.Abs(directory)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(absolute)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("local repository path %q is not a directory", absolute)
		}
		for _, reserved := range []string{"/build", "/etc", "/var/cache/pacman/pkg"} {
			if pathsOverlap(absolute, reserved) {
				return nil, fmt.Errorf("local repository path %q overlaps container path %q", absolute, reserved)
			}
		}
		paths = append(paths, absolute)
	}
	slices.SortFunc(paths, func(left, right string) int {
		if difference := len(left) - len(right); difference != 0 {
			return difference
		}
		return strings.Compare(left, right)
	})
	selected := make([]string, 0, len(paths))
	for _, candidate := range paths {
		contained := false
		for _, parent := range selected {
			if pathContains(parent, candidate) {
				contained = true
				break
			}
		}
		if !contained {
			selected = append(selected, candidate)
		}
	}
	mounts := make([]mount.Mount, 0, len(selected))
	for _, directory := range selected {
		mounts = append(mounts, mount.Mount{
			Type: mount.TypeBind, Source: directory, Target: directory, ReadOnly: true,
		})
	}
	return mounts, nil
}

func pathsOverlap(left, right string) bool {
	return pathContains(left, right) || pathContains(right, left)
}

func pathContains(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (b *Backend) cacheMounts() ([]mount.Mount, error) {
	var mounts []mount.Mount
	add := func(hostDir, target string) error {
		if hostDir == "" {
			return nil
		}
		abs, err := filepath.Abs(hostDir)
		if err != nil {
			return failure.Wrap(err, "failed to resolve cache dir")
		}
		if err := os.MkdirAll(abs, 0o755); err != nil { //nolint:gosec // cache dir shared with the build container
			return failure.Wrap(err, "failed to create cache dir")
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

func installNamesCommand(packages []string) (string, error) {
	if len(packages) == 0 {
		return "", nil
	}
	names := make([]string, 0, len(packages))
	for _, name := range packages {
		if name == "" || strings.HasPrefix(name, "-") || strings.ContainsAny(name, " \t\r\n") {
			return "", fmt.Errorf("invalid install package name %q", name)
		}
		names = append(names, shell.Quote(name))
	}
	return "pacman -S --needed --noconfirm -- " + strings.Join(names, " ") + "\n", nil
}
