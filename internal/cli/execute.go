package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/cli/version"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// UsageError marks a command-line usage mistake (bad flag or arguments) so
// Execute can exit 2 instead of 1.
type UsageError struct{ Err error }

func (e *UsageError) Error() string { return e.Err.Error() }
func (e *UsageError) Unwrap() error { return e.Err }

// SetVersion enables cobra's --version flag on a root, reporting the same build
// info as the version subcommand.
func SetVersion(root *cobra.Command) {
	root.Version = version.String()
}

func NoArgs(cmd *cobra.Command, args []string) error {
	if err := cobra.NoArgs(cmd, args); err != nil {
		return &UsageError{Err: err}
	}
	return nil
}

// Execute runs root, prints failures once, and returns the process exit code.
func Execute(root *cobra.Command) int {
	root.SilenceErrors = true
	root.SilenceUsage = true
	enableGroupHelp(root)
	markArgumentErrors(root)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &UsageError{Err: err}
	})
	err := root.Execute()
	if err == nil {
		return 0
	}
	_, _ = fmt.Fprintf(root.ErrOrStderr(), "Error: %v\n", err)
	var usage *UsageError
	if errors.As(err, &usage) || isCobraUsageError(err) {
		return 2
	}
	return 1
}

func enableGroupHelp(root *cobra.Command) {
	for _, child := range root.Commands() {
		if !child.Runnable() && child.HasSubCommands() {
			child.Args = NoArgs
			child.RunE = func(cmd *cobra.Command, args []string) error {
				if err := NoArgs(cmd, args); err != nil {
					return err
				}
				return cmd.Help()
			}
		}
		enableGroupHelp(child)
	}
}

func isCobraUsageError(err error) bool {
	message := err.Error()
	return strings.HasPrefix(message, "unknown command ") ||
		strings.HasPrefix(message, "required flag(s) ") ||
		strings.HasPrefix(message, "if any flags in the group ") ||
		strings.HasPrefix(message, "at least one of the flags in the group ")
}

func markArgumentErrors(cmd *cobra.Command) {
	if validate := cmd.Args; validate != nil {
		cmd.Args = func(cmd *cobra.Command, args []string) error {
			err := validate(cmd, args)
			if err == nil {
				return nil
			}
			var usage *UsageError
			if errors.As(err, &usage) {
				return err
			}
			return &UsageError{Err: err}
		}
	}
	for _, child := range cmd.Commands() {
		markArgumentErrors(child)
	}
}
