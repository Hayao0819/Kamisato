package servercmd

import (
	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/spf13/cobra"
)

func AddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <username> <password>",
		Short: "Add a new server",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := blinkyutils.ReadServerDB()
			if err != nil {
				return err
			}
			name, user, pass := args[0], args[1], args[2]
			db.Servers[name] = blinkyutils.Server{Username: user, Password: pass}
			return blinkyutils.SaveServerDB(db)
		},
	}
}
