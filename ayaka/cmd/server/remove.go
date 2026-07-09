package servercmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
)

func RemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <server>",
		Short: "Remove a server from the local registry",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return blinkyutils.ServerNames(toComplete), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := blinkyutils.ReadServerDB()
			if err != nil {
				return err
			}
			delete(db.Servers, args[0])
			blinkyutils.ForgetSecret(args[0])
			if db.DefaultServer == args[0] {
				db.DefaultServer = ""
			}
			return blinkyutils.SaveServerDB(db)
		},
	}
	return cmd
}
