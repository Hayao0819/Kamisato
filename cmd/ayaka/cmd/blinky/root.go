package blinkycmd

import (
	"github.com/spf13/cobra"
)

var subCmds []*cobra.Command

func Root() *cobra.Command {
	cmd := cobra.Command{
		Use:   "blinky",
		Short: "Connect to blinky and control it",
	}

	cmd.AddCommand(subCmds...)

	return &cmd
}
