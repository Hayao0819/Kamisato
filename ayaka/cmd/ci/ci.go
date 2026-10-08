// Package cicmd groups the commands that exist for CI pipelines rather than
// for a human at a terminal: computing what a run must build and shaping it
// into job matrices.
package cicmd

import (
	matrixcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/ci/matrix"
	nvcheckcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/ci/nvcheck"
	plancmd "github.com/Hayao0819/Kamisato/ayaka/cmd/ci/plan"
	"github.com/spf13/cobra"
)

// Cmd builds the `ayaka ci` command group.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ci",
		Short: "Commands for CI pipelines (plan, matrix, nvcheck)",
	}
	cmd.AddCommand(
		plancmd.Cmd(),
		matrixcmd.Cmd(),
		nvcheckcmd.Cmd(),
	)
	return cmd
}
