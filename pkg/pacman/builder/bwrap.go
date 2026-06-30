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

	utils "github.com/Hayao0819/Kamisato/internal/utils"
)

// bwrapDepsScript is the phase-1 (root-in-userns) entrypoint: it installs the
// PKGBUILD's dependencies into the overlay. __INSTALL__ is substituted per build.
//
//go:embed bwrap_deps.sh
var bwrapDepsScript string

// bwrapBuildScript is the phase-2 (non-root) entrypoint: deps are already present,
// so makepkg builds without --syncdeps.
const bwrapBuildScript = "set -e\ncd /build\nmakepkg --noconfirm --log --holdver\n"

// bwrapBuildUID is the unprivileged uid the build phase maps to. The host uid is
// mapped 1:1 to it inside the user namespace.
const bwrapBuildUID = "1000"

// bwrapBackend builds packages in a rootless bubblewrap clean room: a pristine
// Arch rootfs (read-only lower) with a throwaway per-build overlay upper. Because
// rootless bwrap maps only a single uid, the build runs in two phases over the
// shared overlay — phase 1 (uid 0) installs deps via pacman, phase 2 (uid 1000)
// runs makepkg, which refuses to run as root.
//
// It is host-only and rootless: it refuses to run nested inside a container, and
// needs no root or daemon, only host-enabled unprivileged user namespaces.
//
// NOTE: this path needs validation on a real Arch host (bwrap >= 0.11 for
// --overlay, unprivileged overlayfs, a keyring-populated rootfs); it cannot run
// in this CI sandbox. Cross-arch (CARCH override / qemu) is a TODO.
type bwrapBackend struct {
	rootfs  string
	timeout time.Duration
}

func newBwrapBackend(opts Options) Backend {
	return &bwrapBackend{rootfs: opts.BwrapRootfs, timeout: opts.Timeout}
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
			"run miko on the host (TODO: nested bwrap needs the outer container's seccomp/AppArmor relaxed)", reason)
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
		return nil, utils.WrapErr(err, "failed to resolve src dir")
	}

	// Overlay scratch (upper + per-phase work dirs) must live on a real fs that
	// can host an overlayfs upper, so place it next to the rootfs rather than in
	// a possibly-tmpfs TMPDIR.
	scratch, err := os.MkdirTemp(filepath.Dir(b.rootfs), "bwrap-build-")
	if err != nil {
		return nil, utils.WrapErr(err, "failed to create overlay scratch dir")
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	upper := filepath.Join(scratch, "upper")
	work1 := filepath.Join(scratch, "work1")
	work2 := filepath.Join(scratch, "work2")
	for _, d := range []string{upper, work1, work2} {
		if err := os.Mkdir(d, 0o755); err != nil {
			return nil, utils.WrapErr(err, "failed to create overlay dir")
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

	depsScript, installBinds := bwrapInstall(spec.InstallPkgs)

	// Phase 1: install deps as root-in-userns over the shared overlay.
	slog.Info("bwrap phase 1: installing dependencies", "dir", srcAbs, "rootfs", b.rootfs)
	p1 := bwrapArgs(b.rootfs, upper, work1, srcAbs, "0", depsScript, installBinds)
	if err := runBwrap(ctx, p1, out); err != nil {
		return nil, utils.WrapErr(err, "bwrap dependency phase failed (ensure unprivileged user namespaces and bwrap >= 0.11 with overlay support)")
	}

	// Phase 2: build as the unprivileged user over the same overlay (deps present).
	slog.Info("bwrap phase 2: building package", "dir", srcAbs, "arch", spec.Arch)
	p2 := bwrapArgs(b.rootfs, upper, work2, srcAbs, bwrapBuildUID, bwrapBuildScript, nil)
	if err := runBwrap(ctx, p2, out); err != nil {
		return nil, utils.WrapErr(err, "bwrap build phase failed")
	}

	built, err := collectNewPackages(srcAbs, baseline)
	if err != nil {
		return nil, err
	}
	if len(built) == 0 {
		return nil, errors.New("no package files (*.pkg.tar.*) were produced")
	}
	return moveToOutDir(built, srcAbs, spec.OutDir)
}

// bwrapInstall turns local InstallPkgs (build-chain deps not yet published) into
// read-only binds plus the pacman -U lines substituted into the deps script.
func bwrapInstall(installPkgs []string) (script string, binds [][2]string) {
	var cmds strings.Builder
	for i, p := range installPkgs {
		dest := fmt.Sprintf("/build/install/%d-%s", i, filepath.Base(p))
		binds = append(binds, [2]string{p, dest})
		// Shell-quote the install path so a hostile filename can't inject into
		// the bash -c deps script, mirroring the container backend.
		fmt.Fprintf(&cmds, "pacman -U --noconfirm %s\n", shellQuote(dest))
	}
	return strings.ReplaceAll(bwrapDepsScript, "__INSTALL__", strings.TrimRight(cmds.String(), "\n")), binds
}

// bwrapArgs builds the sandbox for one phase: the rootfs as a read-only overlay
// lower with a writable upper, the build dir bound at /build, private dev/proc/tmp,
// and fresh ipc/pid/uts/cgroup namespaces. The network is left shared so pacman
// and makepkg can reach mirrors and fetch sources. uid maps the host user 1:1 to
// the given inner uid (0 for the dep phase, unprivileged for the build phase).
func bwrapArgs(rootfs, upper, work, srcAbs, uid, script string, installBinds [][2]string) []string {
	args := []string{
		"--unshare-user", "--unshare-ipc", "--unshare-pid", "--unshare-uts", "--unshare-cgroup",
		"--uid", uid, "--gid", uid,
		"--die-with-parent", "--new-session",
		"--overlay-src", rootfs, "--overlay", upper, work, "/",
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--bind", srcAbs, "/build",
		"--chdir", "/build",
		"--setenv", "HOME", "/build",
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
	cmd := exec.CommandContext(ctx, "bwrap", args...)
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
