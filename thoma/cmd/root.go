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
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/Hayao0819/Kamisato/internal/version"
	"github.com/Hayao0819/Kamisato/thoma/build"
)

// RootCmd builds the thoma command. The makepkg flags thoma reacts to are
// declared on the command like any cobra program, but DisableFlagParsing is kept
// on because thoma is a shim: it must hand the whole, unreordered argv to the
// real makepkg on passthrough, and let query flags such as --help/--version and
// makepkg's own flags fall through untouched. run therefore parses the flags
// explicitly and reads them via cmd.Flags().
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
	f.StringP("buildscript", "p", "PKGBUILD", "makepkg --buildscript")
	f.StringP("dir", "D", "", "makepkg --dir")
	f.BoolP("ignorearch", "A", false, "makepkg --ignorearch")
	f.Bool("check", false, "makepkg --check")
	f.Bool("nocheck", false, "makepkg --nocheck")
	f.Bool("noverify", false, "makepkg --noverify")
	f.Bool("skipchecksums", false, "makepkg --skipchecksums")
	f.Bool("skipinteg", false, "makepkg --skipinteg")
	f.Bool("skippgpcheck", false, "makepkg --skippgpcheck")
	f.BoolP("install", "i", false, "makepkg --install")
	f.Bool("sign", false, "makepkg --sign")
	f.Bool("nosign", false, "makepkg --nosign")
	f.String("key", "", "makepkg --key")
	f.BoolP("nodeps", "d", false, "makepkg --nodeps")
	f.BoolP("clean", "c", false, "makepkg --clean")
	f.BoolP("cleanbuild", "C", false, "makepkg --cleanbuild")
	f.BoolP("force", "f", false, "makepkg --force")
	f.BoolP("noextract", "e", false, "makepkg --noextract")
	f.BoolP("log", "L", false, "makepkg --log")
	f.BoolP("nocolor", "m", false, "makepkg --nocolor")
	f.Bool("noprepare", false, "makepkg --noprepare")
	f.Bool("holdver", false, "makepkg --holdver")
	f.BoolP("rmdeps", "r", false, "makepkg --rmdeps")
	f.BoolP("syncdeps", "s", false, "makepkg --syncdeps")
	f.Bool("asdeps", false, "makepkg --asdeps")
	f.Bool("needed", false, "makepkg --needed")
	f.Bool("noconfirm", false, "makepkg --noconfirm")
	f.Bool("noprogressbar", false, "makepkg --noprogressbar")
	f.BoolP("repackage", "R", false, "makepkg --repackage")
	f.Bool("noarchive", false, "makepkg --noarchive")
	f.SetOutput(io.Discard)
	return cmd
}

func run(cmd *cobra.Command, args []string) error {
	// DisableFlagParsing rules out a cobra `version` subcommand, so intercept the
	// bare verb here; makepkg never takes a "version" positional.
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintf(cmd.OutOrStdout(), "thoma version %s\n", version.String())
		return nil
	}
	// Parse the raw argv into the command's own flags for classification; makepkg's
	// args stays intact for passthrough.
	f := cmd.Flags()
	if err := f.Parse(args); err != nil {
		return err
	}
	if isRemoteBuild(f) {
		options, err := remoteBuildOptions(f)
		if err != nil {
			return err
		}
		if err := rejectRoot(os.Geteuid()); err != nil {
			return err
		}
		return build.Run(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), options)
	}
	return passthrough(args)
}

func remoteBuildOptions(f *pflag.FlagSet) (build.Options, error) {
	for _, name := range []string{"install", "sign", "nosign", "key", "nodeps", "log"} {
		if f.Changed(name) {
			return build.Options{}, fmt.Errorf("thoma remote build does not support --%s", name)
		}
	}
	check, _ := f.GetBool("check")
	nocheck, _ := f.GetBool("nocheck")
	if check && nocheck {
		return build.Options{}, fmt.Errorf("--check and --nocheck are mutually exclusive")
	}
	var runCheck *bool
	if check || nocheck {
		value := check
		runCheck = &value
	}
	var runVerify *bool
	if f.Changed("noverify") {
		value := false
		runVerify = &value
	}
	skipInteg, _ := f.GetBool("skipinteg")
	skipChecksums, _ := f.GetBool("skipchecksums")
	skipPGP, _ := f.GetBool("skippgpcheck")
	config, _ := f.GetString("config")
	buildscript, _ := f.GetString("buildscript")
	dir, _ := f.GetString("dir")
	ignoreArch, _ := f.GetBool("ignorearch")
	return build.Options{
		Config:        config,
		Buildscript:   buildscript,
		Dir:           dir,
		IgnoreArch:    ignoreArch,
		RunCheck:      runCheck,
		RunVerify:     runVerify,
		SkipChecksums: skipInteg || skipChecksums,
		SkipPGPCheck:  skipInteg || skipPGP,
	}, nil
}

func rejectRoot(euid int) error {
	if euid == 0 {
		return fmt.Errorf("running makepkg as root is not allowed as it can cause permanent, catastrophic damage to your system")
	}
	return nil
}

func passthrough(args []string) error {
	bin := build.RealMakepkg("")
	return syscall.Exec(bin, append([]string{bin}, args...), os.Environ()) //nolint:gosec // bin is a resolved makepkg path, not attacker input
}

// nonBuildFlags name the makepkg flags whose presence marks a query/download
// step rather than the heavy compile, so thoma runs it locally. --config is
// deliberately excluded: it can accompany a real build.
var nonBuildFlags = []string{
	"nobuild", "verifysource", "packagelist", "printsrcinfo",
	"source", "allsource", "geninteg", "version", "help", "repackage", "noarchive",
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
