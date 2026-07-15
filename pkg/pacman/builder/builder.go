package builder

import (
	"io"
	"time"
)

// Target is the build config for a local build, mapping to a Spec the chosen backend consumes.
type Target struct {
	Arch        string
	ArchBuild   string
	SignKey     string
	InstallPkgs []string
	// Repos are per-build pacman repositories from repo.json build.repos.
	Repos []RepoSpec
	// Makepkg carries per-build makepkg.conf overrides from repo.json build.makepkg.
	Makepkg MakepkgSettings
	// Image overrides the container image (repo.json build.image), e.g. a 32-bit
	// archlinux32 base for the pentium4/i686/i486 targets.
	Image string
	// Executor selects the build backend. Empty means chroot.
	Executor Kind
	// Timeout bounds a single package build (repo.json build.timeout or the
	// --timeout flag). Zero means the backend default.
	Timeout time.Duration
	// Output, when non-nil, captures the build's stdout/stderr instead of os.Stdout.
	Output io.Writer
}
