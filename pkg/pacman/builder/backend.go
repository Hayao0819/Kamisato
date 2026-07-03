package builder

import (
	"context"
	"fmt"
	"io"
	"time"
)

// Spec describes a single package build, independent of the backend used
// (clean chroot via devtools, or a throwaway container).
type Spec struct {
	// SrcDir is a directory containing the PKGBUILD and any local sources.
	SrcDir string
	// OutDir is where built package files are collected. It may equal SrcDir.
	OutDir string
	// Arch is the target CARCH (x86_64, aarch64, armv7h, ...).
	Arch string
	// Microarch, when set, targets an x86-64 feature level (x86_64_v2/v3/v4): the
	// backend injects the matching -march into the build's makepkg.conf. Empty
	// builds at the arch's baseline. Only the container backend honours it.
	Microarch string
	// ArchBuild is the devtools wrapper used by the chroot backend
	// (e.g. extra-x86_64-build). The container backend ignores it.
	ArchBuild string
	// InstallPkgs are local package files installed into the build environment
	// before building (makechrootpkg -I / pacman -U), for not-yet-published
	// build-chain dependencies.
	InstallPkgs []string
	// LogWriter, when non-nil, receives the build's combined stdout/stderr in
	// addition to the process console. Used by callers (e.g. miko) to capture
	// per-job build logs.
	LogWriter io.Writer
}

// Result reports what a build produced.
type Result struct {
	// Packages are absolute paths to the built .pkg.tar.* files.
	Packages []string
}

// Backend builds a package from source into package files. It does not sign or
// upload the result; those are separate stages owned by the caller.
type Backend interface {
	// Name returns the backend identifier ("chroot", "container").
	Name() string
	Build(ctx context.Context, spec Spec) (*Result, error)
}

// Kind identifies a build backend implementation.
type Kind string

const (
	// KindChroot builds in a clean chroot via devtools (Arch host, root/nspawn).
	KindChroot Kind = "chroot"
	// KindContainer builds in a throwaway container (distro-independent).
	KindContainer Kind = "container"
	// KindBwrap builds in a bubblewrap sandbox over the host toolchain. Host-only
	// (rootless user namespaces); refuses to run nested inside a container.
	KindBwrap Kind = "bwrap"
)

// Options configures backend construction. Fields a backend does not use are
// ignored by it.
type Options struct {
	// Image is the container image for the container backend.
	// Empty means the backend default (archlinux:latest).
	Image string
	// Timeout bounds a single build. Zero means the backend default.
	Timeout time.Duration
	// DockerHost overrides the Docker daemon endpoint for the container backend.
	// Empty falls back to DOCKER_HOST, then the active docker context, then the
	// default socket.
	DockerHost string
	// PacmanCacheDir, when set, is bind-mounted at /var/cache/pacman/pkg by the
	// container and bwrap backends to persist packages across builds and resume
	// interrupted downloads (chroot ignores it).
	PacmanCacheDir string
	// CcacheDir, when set, is bind-mounted at /build/ccache by the container
	// backend to persist a compiler cache across builds (chroot ignores it).
	CcacheDir string
	// BwrapRootfs is the path to a pristine Arch rootfs (pacstrap'd, with a
	// populated pacman keyring) used as the read-only lower layer by the bwrap
	// backend. Required for KindBwrap.
	BwrapRootfs string
	// ExtraRepos are pacman repositories added to the build environment (e.g. the
	// ayato repo) so already-published dependencies resolve during the build. The
	// container and bwrap backends inject them into /etc/pacman.conf; the chroot
	// backend does not (use InstallPkgs for its build-chain dependencies).
	ExtraRepos []RepoSpec
}

// New returns a Backend for the given kind. An empty kind defaults to chroot,
// preserving the historical local-build behaviour.
func New(kind Kind, opts Options) (Backend, error) {
	switch kind {
	case KindChroot, "":
		return newChrootBackend(opts), nil
	case KindContainer:
		return newContainerBackend(opts), nil
	case KindBwrap:
		return newBwrapBackend(opts), nil
	default:
		return nil, fmt.Errorf("unknown build backend: %q", kind)
	}
}
