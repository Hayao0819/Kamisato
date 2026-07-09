// Package cmd implements thoma, a drop-in makepkg replacement that offloads the
// heavy build to a remote miko builder (directly, or via ayato). Point an AUR
// helper at it — e.g. yay's `makepkgbin = thoma` — and the compile happens on
// the build server while the rest of makepkg's work (source download, .SRCINFO,
// package list) passes through to the real makepkg locally. It exists for
// low-powered machines that want to keep using yay without compiling locally.
package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/Hayao0819/Kamisato/internal/version"
)

// RootCmd builds the thoma command. The makepkg flags thoma reacts to are
// declared on the command like any cobra program, but DisableFlagParsing is kept
// on because thoma is a shim: it must hand the whole, unreordered argv to the
// real makepkg on passthrough, and let query flags such as --help/--version and
// makepkg's own flags fall through untouched. run therefore parses the flags
// explicitly (whitelisting makepkg's as unknown) and reads them via cmd.Flags().
func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "thoma [makepkg args...]",
		Short:              "A makepkg drop-in that builds on a remote miko builder",
		DisableFlagParsing: true,
		SilenceErrors:      true,
		SilenceUsage:       true,
		RunE:               run,
	}
	f := cmd.Flags()
	f.BoolP("nobuild", "o", false, "makepkg --nobuild")
	f.Bool("verifysource", false, "makepkg --verifysource")
	f.Bool("packagelist", false, "makepkg --packagelist")
	f.Bool("printsrcinfo", false, "makepkg --printsrcinfo")
	f.BoolP("source", "S", false, "makepkg --source")
	f.Bool("allsource", false, "makepkg --allsource")
	f.BoolP("geninteg", "g", false, "makepkg --geninteg")
	f.BoolP("version", "V", false, "makepkg --version")
	f.BoolP("help", "h", false, "makepkg --help")
	f.String("config", "", "makepkg --config")
	f.ParseErrorsWhitelist.UnknownFlags = true
	f.SetOutput(io.Discard)
	return cmd
}

func run(cmd *cobra.Command, args []string) error {
	// DisableFlagParsing rules out a cobra `version` subcommand, so intercept the
	// bare verb here; makepkg never takes a "version" positional.
	if len(args) == 1 && args[0] == "version" {
		fmt.Printf("thoma version %s\n", version.String())
		return nil
	}
	// Parse the raw argv into the command's own flags for classification; makepkg's
	// flags are whitelisted as unknown and args stays intact for passthrough.
	f := cmd.Flags()
	_ = f.Parse(args)
	if isRemoteBuild(f) {
		config, _ := f.GetString("config")
		return remoteBuild(config)
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
	fa, err := os.Stat(a) //nolint:gosec // a is a resolved makepkg binary path (LookPath/hardcoded), not attacker input
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
	return syscall.Exec(bin, append([]string{bin}, args...), os.Environ()) //nolint:gosec // bin is a resolved makepkg path, not attacker input
}

// nonBuildFlags name the makepkg flags whose presence marks a query/download
// step rather than the heavy compile, so thoma runs it locally. --config is
// deliberately excluded: it can accompany a real build.
var nonBuildFlags = []string{
	"nobuild", "verifysource", "packagelist", "printsrcinfo",
	"source", "allsource", "geninteg", "version", "help",
}

// isRemoteBuild reports whether the invocation is the actual compile+package
// step — the only one worth sending to the remote builder. yay calls makepkg
// separately to download sources (--verifysource), extract/bump pkgver
// (--nobuild), and list outputs (--packagelist); those, and query flags, stay
// local. f has already parsed the argv, so a bundled short cluster like -ofA has
// its -o recorded as Changed.
func isRemoteBuild(f *pflag.FlagSet) bool {
	for _, name := range nonBuildFlags {
		if f.Changed(name) {
			return false
		}
	}
	return true
}
