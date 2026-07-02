// Package version exposes the build version, injected at release time through
// -ldflags -X and otherwise recovered from the module build info so a plain
// `go install`/`go build` still reports something useful.
package version

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// BuildInfo returns the version, commit and build date. When the vars are still
// their defaults (no -ldflags override), it falls back to the VCS stamps the Go
// toolchain embeds via runtime/debug.
func BuildInfo() (version, commit, date string) {
	version, commit, date = Version, Commit, Date
	if version != "dev" {
		return version, commit, date
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version, commit, date
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if commit == "none" {
				commit = s.Value
			}
		case "vcs.time":
			if date == "unknown" {
				date = s.Value
			}
		}
	}
	return version, commit, date
}

// String renders the build info as "<version> (<commit>, <date>)".
func String() string {
	v, c, d := BuildInfo()
	return fmt.Sprintf("%s (%s, %s)", v, c, d)
}

// Command is the shared `version` subcommand every CLI mounts on its root.
func Command() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build version information",
		Run: func(cmd *cobra.Command, _ []string) {
			v, c, d := BuildInfo()
			fmt.Fprintf(cmd.OutOrStdout(), "%s version %s\ncommit: %s\nbuilt:  %s\n", cmd.Root().Name(), v, c, d)
		},
	}
}
