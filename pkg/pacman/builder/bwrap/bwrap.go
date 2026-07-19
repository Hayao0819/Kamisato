package bwrap

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder/internal/artifact"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder/internal/buildenv"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder/internal/errutil"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder/internal/shellutil"
)

//go:embed bwrap_deps.sh
var bwrapDepsScript string

const bwrapBuildScript = "set -e\ncd /build\nmakepkg --noconfirm --log --holdver\n"

const bwrapBuildScriptOverride = "set -e\ncd /build\nmakepkg --config /build/makepkg.override.conf --noconfirm --log --holdver\n"

const bwrapBuildUID = "1000"

// Backend separates dependency installation and makepkg because each rootless namespace maps one UID.
type Backend struct {
	rootfs     string
	timeout    time.Duration
	extraRepos []builder.PacmanRepository
	cacheDir   string
	makepkg    builder.MakepkgConfig
}

func New(config builder.ResolvedConfig) *Backend {
	return &Backend{
		rootfs:     config.Bwrap.Rootfs,
		timeout:    config.Timeout,
		extraRepos: config.Repositories,
		cacheDir:   config.Bwrap.PacmanCacheDir,
		makepkg:    config.Makepkg,
	}
}

func (b *Backend) Name() string { return "bwrap" }

func (b *Backend) Build(ctx context.Context, spec builder.Spec) (*builder.Result, error) {
	if spec.SrcDir == "" {
		return nil, errors.New("bwrap backend requires Spec.SrcDir")
	}
	if b.rootfs == "" {
		return nil, errors.New("bwrap backend requires a pristine Arch rootfs (Config.Bwrap.Rootfs)")
	}
	fi, err := os.Stat(b.rootfs)
	if err != nil {
		return nil, fmt.Errorf("bwrap rootfs %q is not accessible: %w", b.rootfs, err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("bwrap rootfs %q is not a directory", b.rootfs)
	}
	if reason, in := inContainer(); in {
		return nil, fmt.Errorf("bwrap backend is host-only and refuses to run inside a container (%s); "+
			"run miko on the host (nested bwrap needs the outer container's seccomp/AppArmor relaxed)", reason)
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		return nil, fmt.Errorf("bwrap backend requires the 'bubblewrap' package (>= 0.11) on PATH: %w", err)
	}

	if b.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, b.timeout)
		defer cancel()
	}

	srcAbs, err := filepath.Abs(spec.SrcDir)
	if err != nil {
		return nil, errutil.Wrap(err, "failed to resolve src dir")
	}
	cacheDir, err := prepareCacheDir(b.cacheDir)
	if err != nil {
		return nil, err
	}

	// Overlay scratch (per-phase upper + work dirs) must live on a real fs that
	// can host an overlayfs upper, so place it next to the rootfs rather than in
	// a possibly-tmpfs TMPDIR.
	scratch, err := os.MkdirTemp(filepath.Dir(b.rootfs), "bwrap-build-")
	if err != nil {
		return nil, errutil.Wrap(err, "failed to create overlay scratch dir")
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	// depsUpper captures phase 1's dependency install. Phase 2 cannot reuse it as
	// its own overlay upperdir — the kernel refuses to mount an overlay onto an
	// upperdir a prior mount already wrote to (EBUSY, userxattr in-use marker) —
	// so phase 2 stacks it as a read-only lower and writes to its own buildUpper.
	depsUpper := filepath.Join(scratch, "deps-upper")
	work1 := filepath.Join(scratch, "work1")
	buildUpper := filepath.Join(scratch, "build-upper")
	work2 := filepath.Join(scratch, "work2")
	for _, d := range []string{depsUpper, work1, buildUpper, work2} {
		if err := os.Mkdir(d, 0o755); err != nil { //nolint:gosec // build overlay dirs accessed by the build sandbox/overlayfs
			return nil, errutil.Wrap(err, "failed to create overlay dir")
		}
	}

	baseline, err := artifact.Snapshot(srcAbs)
	if err != nil {
		return nil, err
	}

	out := io.Writer(os.Stdout)
	if spec.LogWriter != nil {
		out = io.MultiWriter(os.Stdout, spec.LogWriter)
	}

	depsScript, installBinds, err := bwrapInstall(spec.InstallPkgs, b.extraRepos)
	if err != nil {
		return nil, errutil.Wrap(err, "invalid build repository configuration")
	}

	slog.Info("bwrap phase 1: installing dependencies", "dir", srcAbs, "rootfs", b.rootfs)
	p1 := bwrapArgs([]string{b.rootfs}, depsUpper, work1, srcAbs, cacheDir, "0", depsScript, installBinds)
	if err := runBwrap(ctx, p1, out); err != nil {
		return nil, errutil.Wrap(err, "bwrap dependency phase failed (ensure unprivileged user namespaces and bwrap >= 0.11 with overlay support)")
	}

	buildScript := bwrapBuildScript
	var buildBinds [][2]string
	if !b.makepkg.IsZero() {
		overridePath, cleanupOverride, err := buildenv.StageOverrideConf(b.makepkg)
		if err != nil {
			return nil, err
		}
		defer cleanupOverride()
		buildScript = bwrapBuildScriptOverride
		buildBinds = [][2]string{{overridePath, "/build/makepkg.override.conf"}}
	}

	slog.Info("bwrap phase 2: building package", "dir", srcAbs, "arch", spec.Arch)
	p2 := bwrapArgs([]string{b.rootfs, depsUpper}, buildUpper, work2, srcAbs, cacheDir, bwrapBuildUID, buildScript, buildBinds)
	if err := runBwrap(ctx, p2, out); err != nil {
		return nil, errutil.BuildFailure(ctx, err, "bwrap build phase failed")
	}

	built, err := artifact.Collect(srcAbs, baseline)
	if err != nil {
		return nil, err
	}
	if len(built) == 0 {
		return nil, fmt.Errorf("%w: no package files (*.pkg.tar.*) were produced", builder.ErrBuildFailed)
	}
	packages, err := artifact.MoveToDir(built, srcAbs, spec.OutDir)
	if err != nil {
		return nil, err
	}
	return &builder.Result{Packages: packages}, nil
}

func bwrapInstall(installPkgs []string, extraRepos []builder.PacmanRepository) (script string, binds [][2]string, err error) {
	var cmds strings.Builder
	for i, p := range installPkgs {
		dest := fmt.Sprintf("/build/install/%d-%s", i, filepath.Base(p))
		binds = append(binds, [2]string{p, dest})
		// This path enters bash -c rather than bwrap's argv.
		fmt.Fprintf(&cmds, "pacman -U --noconfirm %s\n", shellutil.Quote(dest))
	}
	reposScript, err := buildenv.ExtraReposScript(extraRepos)
	if err != nil {
		return "", nil, err
	}
	script = buildenv.SubstituteBuildPlaceholders(bwrapDepsScript, reposScript, strings.TrimRight(cmds.String(), "\n"))
	return script, binds, nil
}

// Lowers are ordered bottom-to-top; the network stays shared for mirrors and source downloads.
func bwrapArgs(lowers []string, upper, work, srcAbs, cacheDir, uid, script string, installBinds [][2]string) []string {
	args := []string{
		"--unshare-user", "--unshare-ipc", "--unshare-pid", "--unshare-uts", "--unshare-cgroup",
		"--uid", uid, "--gid", uid,
		"--die-with-parent", "--new-session",
	}
	for _, l := range lowers {
		args = append(args, "--overlay-src", l)
	}
	args = append(args,
		"--overlay", upper, work, "/",
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--bind", srcAbs, "/build",
		"--chdir", "/build",
		"--setenv", "HOME", "/build",
	)
	// Keep package downloads outside the throwaway overlay.
	if cacheDir != "" {
		args = append(args, "--bind", cacheDir, "/var/cache/pacman/pkg")
	}
	// DNS for mirror access; the rootfs may not ship a usable resolv.conf.
	if _, err := os.Stat("/etc/resolv.conf"); err == nil {
		args = append(args, "--ro-bind", "/etc/resolv.conf", "/etc/resolv.conf")
	}
	for _, b := range installBinds {
		args = append(args, "--ro-bind", b[0], b[1])
	}
	return append(args, "bash", "-c", script)
}

func runBwrap(ctx context.Context, args []string, out io.Writer) error {
	cmd := exec.CommandContext(ctx, "bwrap", args...) //nolint:gosec // fixed program bwrap, argv passed as separate args (no shell)
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Env = os.Environ()
	return cmd.Run()
}

func prepareCacheDir(dir string) (string, error) {
	if dir == "" {
		return "", nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", errutil.Wrap(err, "failed to resolve bwrap cache dir")
	}
	if err := os.MkdirAll(abs, 0o755); err != nil { //nolint:gosec // shared package cache must be traversable by the sandbox uid
		return "", errutil.Wrap(err, "failed to create bwrap cache dir")
	}
	return abs, nil
}

// inContainer checks common Docker, podman, nspawn, LXC, and Kubernetes markers.
func inContainer() (string, bool) {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "/.dockerenv present", true
	}
	if v := os.Getenv("container"); v != "" {
		return "container=" + v, true
	}
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		s := string(data)
		for _, marker := range []string{"docker", "containerd", "/lxc/", "kubepods"} {
			if strings.Contains(s, marker) {
				return "cgroup indicates a container", true
			}
		}
	}
	return "", false
}
