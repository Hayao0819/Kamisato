package builder

import "io"

// Target is the build configuration ayaka assembles for a local build: which
// backend to use, the signing key, and the chroot inputs. The backend it maps
// to consumes a Spec; only the chroot backend reads the chroot fields.
type Target struct {
	Arch        string
	ArchBuild   string
	SignKey     string
	InstallPkgs []string
	// Executor selects the build backend. Empty means chroot.
	Executor Kind
	// Output, when non-nil, receives the build command's stdout/stderr instead
	// of os.Stdout. Used to capture build logs.
	Output io.Writer
}
