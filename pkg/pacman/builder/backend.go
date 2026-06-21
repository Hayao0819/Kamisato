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
	// Build runs the build described by spec and returns the produced packages.
	Build(ctx context.Context, spec Spec) (*Result, error)
}

// Kind identifies a build backend implementation.
type Kind string

const (
	// KindChroot builds in a clean chroot via devtools (Arch host, root/nspawn).
	KindChroot Kind = "chroot"
	// KindContainer builds in a throwaway container (distro-independent).
	KindContainer Kind = "container"
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
}

// New returns a Backend for the given kind. An empty kind defaults to chroot,
// preserving the historical local-build behaviour.
func New(kind Kind, opts Options) (Backend, error) {
	switch kind {
	case KindChroot, "":
		return newChrootBackend(opts), nil
	case KindContainer:
		return newContainerBackend(opts), nil
	default:
		return nil, fmt.Errorf("unknown build backend: %q", kind)
	}
}
