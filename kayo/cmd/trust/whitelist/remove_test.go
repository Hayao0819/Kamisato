package whitelistcmd

import (
	"slices"
	"testing"
)

func TestRemoveCommand(t *testing.T) {
	cmd := removeCmd()
	if !slices.Contains(cmd.Aliases, "rm") {
		t.Errorf("aliases = %v, want rm", cmd.Aliases)
	}
	if cmd.Use != "remove <pkgname>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "remove <pkgname>")
	}
}
