package mikocmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
)

// Cmd groups the miko build-service commands. ayaka never talks to miko
// directly: requests go to an ayato endpoint that reverse-proxies to miko, so
// --server names an ayato server.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "miko",
		Short: "Submit and inspect builds on the miko build service",
		Long:  "Submit build jobs to miko (via ayato) and inspect their status and logs.",
	}
	shared.AddPersistentServerFlag(cmd)
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
