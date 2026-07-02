package servercmd

import (
	"github.com/spf13/cobra"
)

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
	)

	return cmd
}
