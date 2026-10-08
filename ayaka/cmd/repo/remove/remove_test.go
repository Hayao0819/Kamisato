package removecmd

import (
	"testing"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/sourcerepos"
	"github.com/Hayao0819/Kamisato/ayaka/source"
	"github.com/spf13/cobra"
)

func TestRejectsInvalidModesBeforeAnyOperation(t *testing.T) {
	cases := [][]string{
		{"repo", "package", "--dry-run"},
		{"repo", "package", "--dry-run=false"},
		{"repo", "package", "--arch", "x86_64"},
		{"repo", "package", "--diff-url", "https://example.test"},
		{"--diff", "source", "ignored-package"},
		{"repo"},
	}
	for _, args := range cases {
		t.Run(args[len(args)-1], func(t *testing.T) {
			command := newCommand(sourcerepos.New(func() ([]*source.SourceRepo, error) {
				t.Fatal("invalid arguments loaded repository input")
				return nil, nil
			}))
			command.RunE = func(*cobra.Command, []string) error {
				t.Fatal("invalid arguments reached destructive execution")
				return nil
			}
			command.SilenceUsage, command.SilenceErrors = true, true
			command.SetArgs(args)
			if err := command.Execute(); err == nil {
				t.Fatalf("accepted invalid arguments %v", args)
			}
		})
	}
}

func TestValidModesReachExecution(t *testing.T) {
	for _, args := range [][]string{{"repo", "package"}, {"--diff", "source", "--dry-run"}} {
		command := newCommand(sourcerepos.Static(nil))
		called := false
		command.RunE = func(*cobra.Command, []string) error { called = true; return nil }
		command.SetArgs(args)
		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
		if !called {
			t.Fatal("valid arguments did not reach execution")
		}
	}
}
