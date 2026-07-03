package cliutil

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestColorEnabledRespectsSignals(t *testing.T) {
	cmd := &cobra.Command{Use: "x", Run: func(*cobra.Command, []string) {}}
	AddNoColorFlag(cmd)

	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")

	if err := cmd.PersistentFlags().Set("no-color", "true"); err != nil {
		t.Fatal(err)
	}
	if ColorEnabled(cmd) {
		t.Error("--no-color should disable color")
	}
	if err := cmd.PersistentFlags().Set("no-color", "false"); err != nil {
		t.Fatal(err)
	}

	t.Setenv("NO_COLOR", "1")
	if ColorEnabled(cmd) {
		t.Error("NO_COLOR should disable color")
	}
	t.Setenv("NO_COLOR", "")

	t.Setenv("TERM", "dumb")
	if ColorEnabled(cmd) {
		t.Error("TERM=dumb should disable color")
	}
	t.Setenv("TERM", "xterm-256color")

	// Under go test stderr is not a terminal, so the TTY check ends false.
	if ColorEnabled(cmd) {
		t.Error("non-TTY stderr should disable color")
	}
}
