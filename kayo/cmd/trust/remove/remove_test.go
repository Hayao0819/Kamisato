package removecmd

import (
	"errors"
	"slices"
	"testing"

	"github.com/spf13/cobra"

	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
)

func TestRemoveCommand(t *testing.T) {
	cmd := Cmd()
	if !slices.Contains(cmd.Aliases, "rm") {
		t.Errorf("aliases = %v, want rm", cmd.Aliases)
	}
	if cmd.Use != "remove [<pkgname>]" {
		t.Errorf("Use = %q, want %q", cmd.Use, "remove [<pkgname>]")
	}
}

func TestRemoveValidatesBeforeExecution(t *testing.T) {
	for _, args := range [][]string{
		{}, {"one", "two"}, {"one", "--maintainer", "aur/user"},
		{"--maintainer", ""}, {"--maintainer", "aur/"},
		{"--maintainer", "/user"}, {"--maintainer", "aur/user/other"},
	} {
		cmd := Cmd()
		called := false
		cmd.RunE = func(_ *cobra.Command, _ []string) error { called = true; return nil }
		cmd.SetArgs(args)
		var usage *cmdline.UsageError
		if err := cmd.Execute(); !errors.As(err, &usage) || called {
			t.Fatalf("args=%v: error=%v, called=%v; want usage error before execution", args, err, called)
		}
	}
}
