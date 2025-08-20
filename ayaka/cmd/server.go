package cmd

import (
	servercmd "github.com/Hayao0819/Kamisato/ayaka/cmd/server"
	"github.com/spf13/cobra"
)

func serverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage Blinky servers",
		Long:  "List, add, remove, and manage Blinky servers.",
	}

	cmd.AddCommand(
		servercmd.ListCmd(),
		servercmd.AddCmd(),
		servercmd.RemoveCmd(),
		servercmd.SetDefaultCmd(),
	)

	return cmd
}

func init() {
	subCmds = append(subCmds, serverCmd())
}
