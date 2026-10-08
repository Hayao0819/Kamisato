package expirecmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRejectsConflictingTargetsBeforeKeyAccess(t *testing.T) {
	for _, allSubkeys := range []string{"--subkeys", "--subkeys=false"} {
		t.Run(allSubkeys, func(t *testing.T) {
			command := Cmd()
			command.SilenceUsage, command.SilenceErrors = true, true
			command.RunE = func(*cobra.Command, []string) error {
				t.Fatal("conflicting flags reached signing-key access")
				return nil
			}
			command.SetArgs([]string{"--expire", "24h", "--subkey", "fingerprint", allSubkeys})
			if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "subkey") {
				t.Fatalf("expected mutually exclusive target error, got %v", err)
			}
		})
	}
}
