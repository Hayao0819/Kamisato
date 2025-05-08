package blinkycmd

import (
	blinky_client_util "github.com/BrenekH/blinky/cmd/blinky/util"
	"github.com/spf13/cobra"
)

func loginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login Server Username Password",
		Short: "Login to blinky",
		Args:  cobra.ExactArgs(3),

		RunE: func(cmd *cobra.Command, args []string) error {
			server, username, password := args[0], args[1], args[2]

			serverDb, err := blinky_client_util.ReadServerDB()
			if err != nil {
				return err
			}

			new_server := serverDb.Servers[server]
			new_server.Username = username
			new_server.Password = password
			serverDb.Servers[server] = new_server

			return blinky_client_util.SaveServerDB(serverDb)
		},
	}
}

func init() {
	subCmds = append(subCmds, loginCmd())
}
