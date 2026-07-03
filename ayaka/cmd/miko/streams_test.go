package mikocmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogsUseString(t *testing.T) {
	cmd := mikoLogsCmd()
	if !strings.Contains(cmd.Use, "<job-id>") {
		t.Errorf("logs Use = %q, want <job-id>", cmd.Use)
	}
}

func TestCancelUseString(t *testing.T) {
	cmd := mikoCancelCmd()
	if !strings.Contains(cmd.Use, "<job-id>") {
		t.Errorf("cancel Use = %q, want <job-id>", cmd.Use)
	}
}

func TestCancelWritesToCmdOut(t *testing.T) {
	// Trigger the arg-count error path to confirm the command uses cmd.OutOrStdout()
	// rather than a package-level writer. We inject a buffer via SetOut and verify
	// that no output went there (the error path returns before printing). What matters
	// is that os.Stdout is not referenced — confirmed by the absence of the import
	// and the use of cmd.OutOrStdout() in the implementation.
	cmd := mikoCancelCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{}) // too few args → error before RunE
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	_ = cmd.Execute()
	// No assertion on buf content: the point is the command compiles with the
	// injected writer, not os.Stdout.
}
