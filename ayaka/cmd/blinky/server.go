package blinkycmd

import (
	blinky_client_util "github.com/BrenekH/blinky/cmd/blinky/util"
	"github.com/spf13/cobra"
)

func serverCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "List servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			serverDb, err := blinky_client_util.ReadServerDB()
			if err != nil {
				return err
			}

			for name, server := range serverDb.Servers {
				cmd.Println(name + "(" + server.Username + ")")
			}

			return nil

		},
	}
}

func init() {
	subCmds = append(subCmds, serverCmd())
}
