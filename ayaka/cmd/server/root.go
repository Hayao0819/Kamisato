package servercmd

import (
	"github.com/spf13/cobra"

	admincmd "github.com/Hayao0819/Kamisato/ayaka/cmd/server/admin"
	"github.com/Hayao0819/Kamisato/internal/serverstore"
)

func completeServerNames(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return serverstore.Names(toComplete), cobra.ShellCompDirectiveNoFileComp
}

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage ayato server endpoints",
		Long:  "Register, list, remove, and set the default ayato server. ayaka talks only to ayato, so this is the single set of endpoints it knows.",
	}

	cmd.AddCommand(
		ListCmd(),
		AddCmd(),
		LoginCmd(),
		LogoutCmd(),
		RevokeCmd(),
		RemoveCmd(),
		SetDefaultCmd(),
		admincmd.Cmd(),
	)

	return cmd
}
