package builder

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
)

// bwrapDepsScript is the phase-1 (root-in-userns) entrypoint: it installs the
// PKGBUILD's dependencies into the overlay. __INSTALL__ is substituted per build.
//
//go:embed bwrap_deps.sh
var bwrapDepsScript string

// bwrapBuildScript is the phase-2 (non-root) entrypoint: deps are already present,
// so makepkg builds without --syncdeps.
const bwrapBuildScript = "set -e\ncd /build\nmakepkg --noconfirm --log --holdver\n"

// bwrapBuildScriptOverride is the phase-2 entrypoint when per-build makepkg
// settings are staged; makepkg reads them from the bind-mounted override.conf.
const bwrapBuildScriptOverride = "set -e\ncd /build\nmakepkg --config /build/makepkg.override.conf --noconfirm --log --holdver\n"

// bwrapBuildUID is the unprivileged uid the build phase maps the host uid 1:1 to inside the user namespace.
const bwrapBuildUID = "1000"

// bwrapBackend builds in a rootless bubblewrap clean room (pristine Arch rootfs as lower, throwaway upper). Two phases because
// rootless bwrap maps only one uid: phase 1 (uid 0) installs deps, phase 2 (uid 1000) runs makepkg. Host-only; refuses containers.
// NOTE: needs validation on a real Arch host (bwrap >= 0.11, unprivileged overlayfs, keyring-populated rootfs). Cross-arch not yet supported.
type bwrapBackend struct {
	rootfs     string
	timeout    time.Duration
	extraRepos []RepoSpec
	cacheDir   string
}

func newBwrapBackend(opts Options) Backend {
	return &bwrapBackend{rootfs: opts.BwrapRootfs, timeout: opts.Timeout, extraRepos: opts.ExtraRepos, cacheDir: opts.PacmanCacheDir}
}

func (b *bwrapBackend) Name() string { return "bwrap" }

func (b *bwrapBackend) Build(ctx context.Context, spec Spec) (*Result, error) {
	if spec.SrcDir == "" {
		return nil, errors.New("bwrap backend requires Spec.SrcDir")
	}
	if b.rootfs == "" {
		return nil, errors.New("bwrap backend requires a pristine Arch rootfs (Options.BwrapRootfs)")
	}
	if fi, err := os.Stat(b.rootfs); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("bwrap rootfs %q is not a directory: %w", b.rootfs, err)
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
		return nil, wrapErr(err, "failed to resolve src dir")
	}

	// Overlay scratch (per-phase upper + work dirs) must live on a real fs that
	// can host an overlayfs upper, so place it next to the rootfs rather than in
	// a possibly-tmpfs TMPDIR.
	scratch, err := os.MkdirTemp(filepath.Dir(b.rootfs), "bwrap-build-")
	if err != nil {
		return nil, wrapErr(err, "failed to create overlay scratch dir")
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
			return nil, wrapErr(err, "failed to create overlay dir")
		}
	}

	baseline, err := snapshotPackages(srcAbs)
	if err != nil {
		return nil, err
	}

	out := io.Writer(os.Stdout)
	if spec.LogWriter != nil {
		out = io.MultiWriter(os.Stdout, spec.LogWriter)
	}

	// Merge the two repo channels: Options.ExtraRepos (miko/server config) first,
	// then Spec.Repos (repo.json build.repos).
	effectiveRepos := append(append([]RepoSpec{}, b.extraRepos...), spec.Repos...)
	depsScript, installBinds := bwrapInstall(spec.InstallPkgs, effectiveRepos)

	// Phase 1: install deps as root-in-userns into depsUpper over the rootfs.
	slog.Info("bwrap phase 1: installing dependencies", "dir", srcAbs, "rootfs", b.rootfs)
	p1 := bwrapArgs([]string{b.rootfs}, depsUpper, work1, srcAbs, b.cacheDir, "0", depsScript, installBinds)
	if err := runBwrap(ctx, p1, out); err != nil {
		return nil, wrapErr(err, "bwrap dependency phase failed (ensure unprivileged user namespaces and bwrap >= 0.11 with overlay support)")
	}

	// Per-build makepkg settings, when set, are staged to a host temp file and
	// bind-mounted into phase 2 so makepkg --config picks them up. A zero Makepkg
	// keeps the plain build script, byte-for-byte unchanged from a default build.
	buildScript := bwrapBuildScript
	var buildBinds [][2]string
	if !spec.Makepkg.isZero() {
		overridePath, cleanupOverride, err := stageOverrideConf(spec.Makepkg)
		if err != nil {
			return nil, err
		}
		defer cleanupOverride()
		buildScript = bwrapBuildScriptOverride
		buildBinds = [][2]string{{overridePath, "/build/makepkg.override.conf"}}
	}

	// Phase 2: build as the unprivileged user, stacking phase 1's deps as a
	// read-only lower (rootfs at the bottom, depsUpper on top) with a fresh
	// writable buildUpper — see the depsUpper comment for why it is not reused.
	slog.Info("bwrap phase 2: building package", "dir", srcAbs, "arch", spec.Arch)
	p2 := bwrapArgs([]string{b.rootfs, depsUpper}, buildUpper, work2, srcAbs, b.cacheDir, bwrapBuildUID, buildScript, buildBinds)
	if err := runBwrap(ctx, p2, out); err != nil {
		return nil, wrapErr(err, "bwrap build phase failed")
	}

	built, err := collectNewPackages(srcAbs, baseline)
	if err != nil {
		return nil, err
	}
	if len(built) == 0 {
		return nil, fmt.Errorf("%w: no package files (*.pkg.tar.*) were produced", ErrBuildFailed)
	}
	return moveToOutDir(built, srcAbs, spec.OutDir)
}

// bwrapInstall turns local InstallPkgs (build-chain deps not yet published) into
// read-only binds plus the pacman -U lines substituted into the deps script, and
// injects any extra repositories into the deps script's pacman.conf edit.
func bwrapInstall(installPkgs []string, extraRepos []RepoSpec) (script string, binds [][2]string) {
	var cmds strings.Builder
	for i, p := range installPkgs {
		dest := fmt.Sprintf("/build/install/%d-%s", i, filepath.Base(p))
		binds = append(binds, [2]string{p, dest})
		// Shell-quote the install path so a hostile filename can't inject into
		// the bash -c deps script, mirroring the container backend.
		fmt.Fprintf(&cmds, "pacman -U --noconfirm %s\n", shellQuote(dest))
	}
	script = substituteBuildPlaceholders(bwrapDepsScript, extraReposScript(extraRepos), strings.TrimRight(cmds.String(), "\n"))
	return script, binds
}

// bwrapArgs builds the sandbox for one phase: the lowers as read-only overlay
// layers (bottom-to-top: the rootfs, plus phase 1's dependency upper for phase 2)
// under the phase's own writable upper/work, the build dir bound at /build,
// private dev/proc/tmp, and fresh ipc/pid/uts/cgroup namespaces. The network is
// left shared so pacman and makepkg can reach mirrors and fetch sources. uid maps
// the host user 1:1 to the given inner uid (0 for the dep phase, unprivileged for
// the build phase).
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
	// A persistent package cache survives the throwaway overlay, so downloads
	// resume across builds and an already-fetched package is reused instead of
	// re-downloaded — the difference between a build that completes on a flaky
	// mirror and one that never does.
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

// inContainer reports whether the process is running inside a container, with a
// short reason. Best-effort: it catches Docker (/.dockerenv), podman/nspawn (the
// "container" env var), and cgroup hints.
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
