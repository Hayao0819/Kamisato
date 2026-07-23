// Package cicmd groups the commands that exist for CI pipelines rather than
// for a human at a terminal: computing what a run must build and shaping it
// into job matrices.
package cicmd

import (
	"github.com/spf13/cobra"

	matrixcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/ci/matrix"
	plancmd "github.com/Hayao0819/Kamisato/ayaka/cmd/ci/plan"
)

// Cmd builds the `ayaka ci` command group.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ci",
		Short: "Commands for CI pipelines (plan, matrix)",
	}
	cmd.AddCommand(
		plancmd.Cmd(),
		matrixcmd.Cmd(),
	)
	return cmd
}
