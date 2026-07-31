// Package cicmd groups the commands that exist for CI pipelines rather than
// for a human at a terminal: computing what a run must build and shaping it
// into job matrices.
package cicmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	matrixcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/ci/matrix"
	nvcheckcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/ci/nvcheck"
	plancmd "github.com/Hayao0819/Kamisato/ayaka/cmd/ci/plan"
)

// Cmd builds the `ayaka ci` command group.
func Cmd(runtime *app.Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ci",
		Short: "Commands for CI pipelines (plan, matrix, nvcheck)",
	}
	cmd.AddCommand(
		plancmd.Cmd(runtime),
		matrixcmd.Cmd(runtime),
		nvcheckcmd.Cmd(runtime),
	)
	return cmd
}
