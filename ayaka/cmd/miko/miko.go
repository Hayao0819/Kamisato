package mikocmd

import (
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/remote"
	buildcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/miko/build"
	cancelcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/miko/cancel"
	jobscmd "github.com/Hayao0819/Kamisato/ayaka/cmd/miko/jobs"
	logscmd "github.com/Hayao0819/Kamisato/ayaka/cmd/miko/logs"
	statscmd "github.com/Hayao0819/Kamisato/ayaka/cmd/miko/stats"
	statuscmd "github.com/Hayao0819/Kamisato/ayaka/cmd/miko/status"
	"github.com/spf13/cobra"
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
	remote.AddPersistentServerFlag(cmd)
	cmd.AddCommand(
		buildcmd.Cmd(),
		jobscmd.Cmd(),
		statuscmd.Cmd(),
		logscmd.Cmd(),
		cancelcmd.Cmd(),
		statscmd.Cmd(),
	)
	return cmd
}
