package cliutil

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/version"
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

// Execute runs root and maps the outcome to a process exit code: 0 on success,
// 2 on usage mistakes, 1 on anything else. Error printing is left to cobra.
func Execute(root *cobra.Command) int {
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &UsageError{Err: err}
	})
	err := root.Execute()
	if err == nil {
		return 0
	}
	var usage *UsageError
	if errors.As(err, &usage) {
		return 2
	}
	return 1
}
