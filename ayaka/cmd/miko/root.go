package mikocmd

import (
	"github.com/spf13/cobra"
)

// Cmd groups the client commands for the miko build service. ayaka never
// talks to miko directly: every request goes to an ayato endpoint, which
// reverse-proxies it to miko. The --server flag therefore names an ayato
// server, and is shared by all miko subcommands.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "miko",
		Short: "Submit and inspect builds on the miko build service",
		Long:  "Submit build jobs to miko (via ayato) and inspect their status and logs.",
	}
	cmd.PersistentFlags().StringP("server", "s", "", "ayato server that relays to miko (default: serverdb default)")
	cmd.AddCommand(
		mikoBuildCmd(),
		mikoJobsCmd(),
		mikoStatusCmd(),
		mikoLogsCmd(),
		mikoCancelCmd(),
		mikoStatsCmd(),
	)
	return cmd
}
