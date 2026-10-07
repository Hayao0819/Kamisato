package cmd

import (
	"testing"

	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
)

func TestAuditRejectsArgumentsBeforePruning(t *testing.T) {
	cmd := auditCmd()
	cmd.SetArgs([]string{"unintended", "--prune"})
	if got := cmdline.Execute(cmd); got != 2 {
		t.Fatalf("exit code = %d, want 2", got)
	}
}
