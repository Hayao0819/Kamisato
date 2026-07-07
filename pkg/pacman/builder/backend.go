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
	// Repos are per-build pacman repositories (repo.json build.repos), merged after Options.ExtraRepos.
	// All backends inject them into pacman.conf; chroot does it via the generated -C config.
	Repos []RepoSpec
	// Makepkg carries per-build makepkg.conf overrides; all backends append them after source.
	// Chroot does it via the generated -M config.
	Makepkg MakepkgSettings
	// ArchBuild is the devtools wrapper (e.g. extra-x86_64-build); with a build config the chroot backend
	// only derives its base repo from it; without one it shells out directly. Container and bwrap ignore it.
	ArchBuild string
	// InstallPkgs are local package files installed before building (makechrootpkg -I / pacman -U)
	// for not-yet-published build-chain dependencies.
	InstallPkgs []string
	// LogWriter, when non-nil, receives the build's combined stdout/stderr (in addition to the console)
	// for per-job log capture.
	LogWriter io.Writer
}

// MakepkgSettings are per-build makepkg.conf overrides rendered after the base;
// each set value replaces (or for CFLAGS/OPTIONS appends to) the distro default. A zero value is a no-op.
type MakepkgSettings struct {
	Packager     string
	Microarch    string
	CFlagsAppend string
	Options      []string
}

func (s MakepkgSettings) isZero() bool {
	return s.Packager == "" && s.Microarch == "" && s.CFlagsAppend == "" && len(s.Options) == 0
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
	// PacmanCacheDir, when set, is bind-mounted at /var/cache/pacman/pkg by container and bwrap
	// to persist packages across builds (chroot ignores it).
	PacmanCacheDir string
	// CcacheDir, when set, is bind-mounted at /build/ccache by the container
	// backend to persist a compiler cache across builds (chroot ignores it).
	CcacheDir string
	// BwrapRootfs is the path to a pristine Arch rootfs (pacstrap'd, with a
	// populated pacman keyring) used as the read-only lower layer by the bwrap
	// backend. Required for KindBwrap.
	BwrapRootfs string
	// ExtraRepos are the server-config pacman repos (e.g. the ayato repo), injected into pacman.conf
	// ahead of Spec.Repos; chroot does it via the generated -C config.
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
