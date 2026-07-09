package cliutil

import (
	"io"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/spf13/cobra"
)

func newRoot(runErr error) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "x",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          func(*cobra.Command, []string) error { return runErr },
	}
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return cmd
}

func TestExecuteExitCodes(t *testing.T) {
	if got := Execute(newRoot(nil)); got != 0 {
		t.Errorf("success: got %d, want 0", got)
	}

	if got := Execute(newRoot(errors.New("boom"))); got != 1 {
		t.Errorf("runtime failure: got %d, want 1", got)
	}

	bad := newRoot(nil)
	bad.SetArgs([]string{"--no-such-flag"})
	if got := Execute(bad); got != 2 {
		t.Errorf("usage error: got %d, want 2", got)
	}
}

func TestUsageErrorUnwrap(t *testing.T) {
	inner := errors.New("inner")
	var usage *UsageError
	if !errors.As(&UsageError{Err: inner}, &usage) || !errors.Is(usage, inner) {
		t.Error("UsageError should unwrap to the original error")
	}
}
