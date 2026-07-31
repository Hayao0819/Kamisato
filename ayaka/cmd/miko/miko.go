package mikocmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/ayaka/cli"
)

// Cmd groups the miko build-service commands. ayaka never talks to miko
// directly: requests go to an ayato endpoint that reverse-proxies to miko, so
// --server names an ayato server.
func Cmd(runtime *app.Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "miko",
		Short: "Submit and inspect builds on the miko build service",
		Long:  "Submit build jobs to miko (via ayato) and inspect their status and logs.",
	}
	cli.AddPersistentServerFlag(cmd)
	cmd.AddCommand(
		mikoBuildCmd(runtime),
		mikoJobsCmd(),
		mikoStatusCmd(),
		mikoLogsCmd(),
		mikoCancelCmd(),
		mikoStatsCmd(),
	)
	return cmd
}
