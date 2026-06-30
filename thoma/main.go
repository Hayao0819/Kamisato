// Command thoma is a drop-in makepkg replacement that offloads the heavy build
// to a remote miko builder (via ayato). Point an AUR helper at it — e.g. yay's
// `makepkgbin = thoma` — and the compile happens on the build server while the
// rest of makepkg's work (source download, .SRCINFO, package list) passes
// through to the real makepkg locally. It exists for low-powered machines that
// want to keep using yay without compiling locally.
package main

import (
	"fmt"
	"os"
	"syscall"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "thoma: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if isRemoteBuild(args) {
		return remoteBuild(args)
	}
	return passthrough(args)
}

// realMakepkg is the makepkg binary thoma delegates pass-through invocations to.
func realMakepkg() string {
	if p := os.Getenv("THOMA_MAKEPKG"); p != "" {
		return p
	}
	return "/usr/bin/makepkg"
}

// passthrough replaces the process with the real makepkg, preserving args, env,
// cwd, stdio and exit code exactly — used for every invocation that is not the
// heavy compile (source download, --nobuild, --packagelist, --printsrcinfo, …).
func passthrough(args []string) error {
	bin := realMakepkg()
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
