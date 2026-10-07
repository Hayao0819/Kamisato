package cli

import (
	"bytes"
	"io"
	"strings"
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

func TestExecutePrintsErrorOnce(t *testing.T) {
	cmd := newRoot(errors.New("boom"))
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	if got := Execute(cmd); got != 1 {
		t.Fatalf("exit code = %d, want 1", got)
	}
	if got := stderr.String(); got != "Error: boom\n" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestNoArgsReturnsUsageError(t *testing.T) {
	err := NoArgs(&cobra.Command{Use: "x"}, []string{"extra"})
	var usage *UsageError
	if !errors.As(err, &usage) || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("error = %v, want UsageError", err)
	}
}

func TestExecuteClassifiesArgumentValidation(t *testing.T) {
	cmd := newRoot(nil)
	cmd.Args = cobra.ExactArgs(1)
	cmd.SetArgs(nil)
	if got := Execute(cmd); got != 2 {
		t.Fatalf("exit code = %d, want 2", got)
	}
}

func TestExecuteClassifiesCobraValidationErrors(t *testing.T) {
	t.Run("unknown command", func(t *testing.T) {
		cmd := &cobra.Command{Use: "x"}
		cmd.AddCommand(&cobra.Command{Use: "known", Run: func(*cobra.Command, []string) {}})
		cmd.SetArgs([]string{"unknown"})
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		if got := Execute(cmd); got != 2 {
			t.Fatalf("exit code = %d, want 2", got)
		}
	})

	t.Run("required flag", func(t *testing.T) {
		cmd := newRoot(nil)
		cmd.Flags().String("required", "", "")
		_ = cmd.MarkFlagRequired("required")
		if got := Execute(cmd); got != 2 {
			t.Fatalf("exit code = %d, want 2", got)
		}
	})
}

func TestExecuteRejectsArgumentsToCommandGroup(t *testing.T) {
	root := &cobra.Command{Use: "x"}
	group := &cobra.Command{Use: "group"}
	group.AddCommand(&cobra.Command{Use: "leaf", Run: func(*cobra.Command, []string) {}})
	root.AddCommand(group)
	root.SetArgs([]string{"group", "typo"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	if got := Execute(root); got != 2 {
		t.Fatalf("exit code = %d, want 2", got)
	}
}
