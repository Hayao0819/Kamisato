// No changes needed: already in English
package servercmd

import (
	blinky_utils "github.com/BrenekH/blinky/cmd/blinky/util"
	"github.com/spf13/cobra"
)

func AddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <username> <password>",
		Short: "Add a new server",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := blinky_utils.ReadServerDB()
			if err != nil {
				return err
			}
			name, user, pass := args[0], args[1], args[2]
			db.Servers[name] = blinky_utils.Server{Username: user, Password: pass}
			return blinky_utils.SaveServerDB(db)
		},
	}
}
