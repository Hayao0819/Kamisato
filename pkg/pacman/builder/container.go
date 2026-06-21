package builder

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// buildScript is the in-container entrypoint. __ARCH__ and __INSTALL__ are
// substituted per build.
//
//go:embed buildscript.sh
var buildScript string

const (
	defaultContainerImage = "archlinux:latest"
	defaultBuildTimeout   = 30 * time.Minute
)

func archToPlatform(arch string) (*ocispec.Platform, error) {
	switch arch {
	case "x86_64":
		return &ocispec.Platform{OS: "linux", Architecture: "amd64"}, nil
	case "aarch64":
		return &ocispec.Platform{OS: "linux", Architecture: "arm64"}, nil
	case "armv7h":
		return &ocispec.Platform{OS: "linux", Architecture: "arm", Variant: "v7"}, nil
	default:
		return nil, fmt.Errorf("unsupported architecture: %s", arch)
	}
}

// platformString renders a Docker platform spec as "os/arch[/variant]".
func platformString(p *ocispec.Platform) string {
	s := p.OS + "/" + p.Architecture
	if p.Variant != "" {
		s += "/" + p.Variant
	}
	return s
}

// shellQuote wraps s in single quotes so it can be embedded safely in an
// `sh -c` command, escaping any embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// containerBackend builds packages in a fresh throwaway container, one per
// build (makecontainerpkg-style clean room).
//
// Cross-arch builds rely on the host having qemu-user-static registered with
// binfmt_misc using the "F" (fix_binary) flag, so the emulated interpreter is
// available inside the freshly created container.
type containerBackend struct {
	image          string
	timeout        time.Duration
	host           string
	pacmanCacheDir string
	ccacheDir      string
}

// newContainerBackend constructs the container build backend.
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
	}
}

// newDockerClient resolves the daemon endpoint with the same priority the
// docker CLI uses: explicit host, then DOCKER_HOST, then the active docker
// context, then the default socket. client.FromEnv alone ignores contexts, so a
// host using Docker Desktop / rootless / a remote context would otherwise hit
// the wrong daemon.
func newDockerClient(host string) (*client.Client, error) {
	opts := []client.Opt{client.WithAPIVersionNegotiation(), client.FromEnv}
	if host == "" && os.Getenv("DOCKER_HOST") == "" {
		host = dockerHostFromContext()
	}
	if host != "" {
		opts = append(opts, client.WithHost(host))
	}
	return client.NewClientWithOpts(opts...)
}

// dockerHostFromContext returns the docker endpoint of the active context by
// reading ~/.docker (config.json + the context metadata store). It returns ""
// for the default context or on any error, so callers fall back to the socket.
func dockerHostFromContext() string {
	dir := os.Getenv("DOCKER_CONFIG")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".docker")
	}

	name := os.Getenv("DOCKER_CONTEXT")
	if name == "" {
		data, err := os.ReadFile(filepath.Join(dir, "config.json"))
		if err != nil {
			return ""
		}
		var cfg struct {
			CurrentContext string `json:"currentContext"`
		}
		if json.Unmarshal(data, &cfg) != nil {
			return ""
		}
		name = cfg.CurrentContext
	}
	if name == "" || name == "default" {
		return ""
	}

	// The context store keys each context by the hex sha256 of its name.
	id := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))
	data, err := os.ReadFile(filepath.Join(dir, "contexts", "meta", id, "meta.json"))
	if err != nil {
		return ""
	}
	var meta struct {
		Endpoints map[string]struct {
			Host string `json:"Host"`
		} `json:"Endpoints"`
	}
	if json.Unmarshal(data, &meta) != nil {
		return ""
	}
	return meta.Endpoints["docker"].Host
}

func (b *containerBackend) Name() string { return "container" }

func (b *containerBackend) Build(ctx context.Context, spec Spec) (*Result, error) {
	if spec.SrcDir == "" {
		return nil, errors.New("container backend requires Spec.SrcDir")
	}
	slog.Info("starting container build", "arch", spec.Arch, "image", b.image)

	platform, err := archToPlatform(spec.Arch)
	if err != nil {
		return nil, utils.WrapErr(err, "failed to resolve platform")
	}
	platformStr := platformString(platform)

	absSrc, err := filepath.Abs(spec.SrcDir)
	if err != nil {
		return nil, utils.WrapErr(err, "failed to resolve src dir")
	}
	outDir := spec.OutDir
	if outDir == "" {
		outDir = spec.SrcDir
	}
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return nil, utils.WrapErr(err, "failed to resolve out dir")
	}
	if err := os.MkdirAll(absOut, 0o755); err != nil {
		return nil, utils.WrapErr(err, "failed to create out dir")
	}

	// Record pre-existing packages so a build into a non-empty OutDir (including
	// OutDir == SrcDir) only reports freshly produced artifacts.
	baseline, err := snapshotPackages(absOut)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()

	cli, err := newDockerClient(b.host)
	if err != nil {
		return nil, utils.WrapErr(err, "failed to create docker client")
	}
	defer cli.Close()

	slog.Info("pulling container image", "image", b.image, "platform", platformStr)
	reader, err := cli.ImagePull(ctx, b.image, image.PullOptions{Platform: platformStr})
	if err != nil {
		return nil, utils.WrapErr(err, "failed to pull image")
	}
	if err := drainPullStream(reader); err != nil {
		return nil, utils.WrapErr(err, "failed to pull image")
	}

	// Mount each install package at a unique path; shell-quote the install
	// path so a hostile filename can't inject into `sh -c`.
	installMounts := make([]mount.Mount, 0, len(spec.InstallPkgs))
	var installCmd strings.Builder
	for i, pkg := range spec.InstallPkgs {
		absPkg, err := filepath.Abs(pkg)
		if err != nil {
			return nil, utils.WrapErr(err, "failed to resolve install package path")
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

	script := buildScript
	script = strings.ReplaceAll(script, "__ARCH__", spec.Arch)
	script = strings.ReplaceAll(script, "__INSTALL__", strings.TrimRight(installCmd.String(), "\n"))

	containerConfig := &container.Config{
		Image:      b.image,
		Cmd:        []string{"sh", "-c", script},
		WorkingDir: "/build",
		Tty:        false,
		User:       "root",
	}

	// src is mounted read-only; the script copies it to a writable work dir so
	// the caller's source tree is never mutated. out collects the artifacts.
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
		return nil, utils.WrapErr(err, "failed to create container")
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
		return nil, utils.WrapErr(err, "failed to start container")
	}

	// Stream container output live: stdcopy demuxes the multiplexed log stream
	// (Tty=false) into a synchronized capture buffer and, when set, the caller's
	// LogWriter, so SSE clients see logs while the build runs. capture feeds the
	// error message on failure.
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
			return nil, fmt.Errorf("build failed with exit code %d:\n%s", status.StatusCode, capture.String())
		}
	}

	pkgs, err := collectNewPackages(absOut, baseline)
	if err != nil {
		return nil, utils.WrapErr(err, "failed to collect built packages")
	}
	slog.Info("container build completed", "packages", len(pkgs))
	return &Result{Packages: pkgs}, nil
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
			return utils.WrapErr(err, "failed to resolve cache dir")
		}
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return utils.WrapErr(err, "failed to create cache dir")
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

// drainPullStream consumes the image-pull progress stream and surfaces any
// error delivered as a JSON message in the body (ImagePull only reports the
// initial request error directly).
func drainPullStream(r io.ReadCloser) error {
	defer r.Close()
	dec := json.NewDecoder(r)
	for {
		var msg struct {
			Error string `json:"error"`
		}
		if err := dec.Decode(&msg); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if msg.Error != "" {
			return errors.New(msg.Error)
		}
	}
}

// syncBuffer is a concurrency-safe bytes.Buffer for the log capture goroutine.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}
