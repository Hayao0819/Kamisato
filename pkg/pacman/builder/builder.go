package builder

import "io"

// Target is the build configuration ayaka assembles for a local build: which
// backend to use, the signing key, and the per-build environment (chroot wrapper,
// extra repos, makepkg overrides). It maps to a Spec the chosen backend consumes.
type Target struct {
	Arch        string
	ArchBuild   string
	SignKey     string
	InstallPkgs []string
	// Repos are per-build pacman repositories from repo.json build.repos.
	Repos []RepoSpec
	// Makepkg carries per-build makepkg.conf overrides from repo.json build.makepkg.
	Makepkg MakepkgSettings
	// Executor selects the build backend. Empty means chroot.
	Executor Kind
	// Output, when non-nil, receives the build command's stdout/stderr instead
	// of os.Stdout. Used to capture build logs.
	Output io.Writer
}
