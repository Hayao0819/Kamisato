package plancmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/cliutil"
)

func TestPlanFlagContract(t *testing.T) {
	command := Cmd()
	for _, name := range []string{"arch", "local-source", "pacman-conf", "backend", "image", "timeout", "work-dir", "keep-work", "json"} {
		if command.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s is missing", name)
		}
	}
	for _, name := range []string{"aur-list", "pkgbuild-root"} {
		if command.Flags().Lookup(name) != nil {
			t.Errorf("flag --%s should not be registered", name)
		}
	}
}

func TestPlanRejectsMissingTargets(t *testing.T) {
	err := executePlan(t, []string{"plan", "--pacman-conf", "ignored"})
	assertUsageError(t, err, "<pkgname>")
}

func TestPlanRejectsMissingPacmanConfig(t *testing.T) {
	err := executePlan(t, []string{"plan", "example"})
	assertUsageError(t, err, "--pacman-conf")
}

func executePlan(t *testing.T, args []string) error {
	t.Helper()
	root := &cobra.Command{Use: "ayaka"}
	root.PersistentFlags().String("config", "", "")
	root.AddCommand(Cmd())
	root.SetArgs(args)
	root.SilenceErrors = true
	root.SilenceUsage = true
	return root.Execute()
}

func assertUsageError(t *testing.T, err error, contains string) {
	t.Helper()
	var usage *cliutil.UsageError
	if !errors.As(err, &usage) || !strings.Contains(err.Error(), contains) {
		t.Fatalf("error = %v", err)
	}
}
