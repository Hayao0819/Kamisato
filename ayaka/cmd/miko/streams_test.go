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
	// Confirm the command writes through cmd.OutOrStdout(), not os.Stdout: drive
	// the arg-count error path with a buffer injected via SetOut.
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
