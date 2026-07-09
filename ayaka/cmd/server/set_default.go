package servercmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func SetDefaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-default <server>",
		Short: "Set the default ayato server",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return blinkyutils.ServerNames(toComplete), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := blinkyutils.ReadServerDB()
			if err != nil {
				return err
			}
			if _, ok := db.Servers[args[0]]; !ok {
				return errors.WrapErr(shared.ErrServerNotFound, args[0])
			}
			db.DefaultServer = args[0]
			return blinkyutils.SaveServerDB(db)
		},
	}
	return cmd
}
