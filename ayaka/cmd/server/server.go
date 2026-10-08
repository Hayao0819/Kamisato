package servercmd

import (
	addcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/server/add"
	admincmd "github.com/Hayao0819/Kamisato/ayaka/cmd/server/admin"
	listcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/server/list"
	logincmd "github.com/Hayao0819/Kamisato/ayaka/cmd/server/login"
	logoutcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/server/logout"
	removecmd "github.com/Hayao0819/Kamisato/ayaka/cmd/server/remove"
	revokecmd "github.com/Hayao0819/Kamisato/ayaka/cmd/server/revoke"
	setdefaultcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/server/set-default"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage ayato server endpoints",
		Long:  "Register, list, remove, and set the default ayato server. ayaka talks only to ayato, so this is the single set of endpoints it knows.",
	}

	cmd.AddCommand(
		listcmd.Cmd(),
		addcmd.Cmd(),
		logincmd.Cmd(),
		logoutcmd.Cmd(),
		revokecmd.Cmd(),
		removecmd.Cmd(),
		setdefaultcmd.Cmd(),
		admincmd.Cmd(),
	)

	return cmd
}
