package cmd

import (
	"testing"

	"github.com/Hayao0819/Kamisato/internal/cliutil"
)

func TestAuditRejectsArgumentsBeforePruning(t *testing.T) {
	cmd := auditCmd()
	cmd.SetArgs([]string{"unintended", "--prune"})
	if got := cliutil.Execute(cmd); got != 2 {
		t.Fatalf("exit code = %d, want 2", got)
	}
}
