package builder

import (
	"context"
	"io"
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
	// InstallPkgs are local package files installed before building (makechrootpkg -I / pacman -U)
	// for not-yet-published build-chain dependencies.
	InstallPkgs []string
	// LogWriter, when non-nil, receives the build's combined stdout/stderr (in addition to the console)
	// for per-job log capture.
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
