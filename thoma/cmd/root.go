// Package cmd implements thoma, a drop-in makepkg replacement that offloads the
// heavy build to a remote miko builder (directly, or via ayato). Point an AUR
// helper at it — e.g. yay's `makepkgbin = thoma` — and the compile happens on
// the build server while the rest of makepkg's work (source download, .SRCINFO,
// package list) passes through to the real makepkg locally. It exists for
// low-powered machines that want to keep using yay without compiling locally.
package cmd

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
)

// RootCmd builds the thoma command. makepkg forwards clustered short flags and
// long flags thoma does not define (-f, --noextract, -ofA); DisableFlagParsing
// hands the whole argv through untouched so cobra never rejects them, and lets
// query flags such as --help/--version fall through to the real makepkg.
func RootCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "thoma [makepkg args...]",
		Short:              "A makepkg drop-in that builds on a remote miko builder",
		DisableFlagParsing: true,
		SilenceErrors:      true,
		SilenceUsage:       true,
		RunE: func(_ *cobra.Command, args []string) error {
			return run(args)
		},
	}
}

func run(args []string) error {
	if isRemoteBuild(args) {
		return remoteBuild(args)
	}
	return passthrough(args)
}

// realMakepkg resolves the makepkg binary thoma delegates pass-through
// invocations to, trying in order: the explicit THOMA_MAKEPKG / config override,
// $PATH, then the canonical /usr/bin/makepkg. A candidate that is the same file
// as this executable is skipped: when thoma is installed *as* makepkg
// (makepkgbin=thoma with /usr/bin/makepkg symlinked to thoma), delegating to it
// would loop forever through syscall.Exec.
func realMakepkg(configured string) string {
	self, _ := os.Executable()
	var candidates []string
	if p := os.Getenv("THOMA_MAKEPKG"); p != "" {
		candidates = append(candidates, p)
	}
	if configured != "" {
		candidates = append(candidates, configured)
	}
	if p, err := exec.LookPath("makepkg"); err == nil {
		candidates = append(candidates, p)
	}
	candidates = append(candidates, "/usr/bin/makepkg")
	for _, c := range candidates {
		if self != "" && sameFile(c, self) {
			continue
		}
		return c
	}
	return "/usr/bin/makepkg"
}

// sameFile reports whether a and b resolve to the same on-disk file, following
// symlinks — so a /usr/bin/makepkg symlink pointing back at the thoma binary is
// caught even though the two paths differ.
func sameFile(a, b string) bool {
	fa, err := os.Stat(a)
	if err != nil {
		return false
	}
	fb, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(fa, fb)
}

// passthrough replaces the process with the real makepkg, preserving args, env,
// cwd, stdio and exit code exactly — used for every invocation that is not the
// heavy compile (source download, --nobuild, --packagelist, --printsrcinfo, …).
func passthrough(args []string) error {
	bin := realMakepkg("")
	return syscall.Exec(bin, append([]string{bin}, args...), os.Environ())
}

// nonBuildFlags mark a makepkg invocation that does something other than the
// heavy compile, so thoma passes it through to the real makepkg instead of
// redirecting it to the remote builder.
var nonBuildFlags = map[string]bool{
	"--verifysource": true,
	"--nobuild":      true, "-o": true,
	"--packagelist":  true,
	"--printsrcinfo": true,
	"--allsource":    true, "-S": true,
	"--source":   true,
	"--geninteg": true, "-g": true,
	"--version": true, "-V": true,
	"--help": true, "-h": true,
}

// isRemoteBuild reports whether the invocation is the actual compile+package
// step — the only one worth sending to the remote builder. yay calls makepkg
// separately to download sources (--verifysource), extract/bump pkgver
// (--nobuild), and list outputs (--packagelist); those, and query flags, stay
// local.
func isRemoteBuild(args []string) bool {
	for _, a := range args {
		if nonBuildFlags[a] {
			return false
		}
		// Catch bundled short flags like -ofA (paru-style): a single-dash cluster
		// containing one of the non-build short options.
		if len(a) > 1 && a[0] == '-' && a[1] != '-' {
			for _, c := range a[1:] {
				switch c {
				case 'o', 'g', 'S', 'V', 'h':
					return false
				}
			}
		}
	}
	return true
}
