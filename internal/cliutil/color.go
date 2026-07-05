package cliutil

import (
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// AddNoColorFlag registers the shared --no-color flag on a root command.
func AddNoColorFlag(root *cobra.Command) {
	root.PersistentFlags().Bool("no-color", false, "Disable colored output")
}

// ColorEnabled decides whether output may use color: off when --no-color was
// passed, NO_COLOR is set (no-color.org), TERM is dumb, or stderr is not a
// terminal. Logs go to stderr, so that is the stream that matters here.
func ColorEnabled(cmd *cobra.Command) bool {
	if f := cmd.Flags().Lookup("no-color"); f != nil {
		if noColor, err := cmd.Flags().GetBool("no-color"); err == nil && noColor {
			return false
		}
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(os.Stderr.Fd())) //nolint:gosec // G115: fd fits in int
}
