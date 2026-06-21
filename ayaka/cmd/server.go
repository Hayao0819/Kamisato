package cmd

import (
	servercmd "github.com/Hayao0819/Kamisato/ayaka/cmd/server"
	"github.com/spf13/cobra"
)

// serverCmd returns the command to manage ayato server endpoints.
func serverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage ayato server endpoints",
		Long:  "Register, list, remove, and set the default ayato server. ayaka talks only to ayato, so this is the single set of endpoints it knows.",
	}

	cmd.AddCommand(
		servercmd.ListCmd(),
		servercmd.AddCmd(),
		serverLoginCmd(),
		servercmd.RemoveCmd(),
		servercmd.SetDefaultCmd(),
	)

	return cmd
}

func init() {
	subCmds.Add(serverCmd())
}
