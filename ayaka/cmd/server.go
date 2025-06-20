package cmd

import (
	"fmt"

	blinky_utils "github.com/BrenekH/blinky/cmd/blinky/util"
	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"
)

func printServerInfo(n string, s blinky_utils.Server, showPassword bool, prefix string) {
	var serverInfo string
	if s.Username == "" {
		serverInfo = fmt.Sprintf("%s%s\n", prefix, n)
	} else if showPassword {
		serverInfo = fmt.Sprintf("%s%s(%s:%s)\n", prefix, n, s.Username, s.Password)
	} else {
		serverInfo = fmt.Sprintf("%s%s(%s)\n", prefix, n, s.Username)
	}
	fmt.Print(serverInfo)
}

func serverCmd() *cobra.Command {
	var serverCmd = cobra.Command{
		Use:   "server [server_url...]",
		Short: "Manage Blinky servers",
		Long: `Manage Blinky servers. This command is used to list the servers
that are saved in the server database.
It can also be used to check the login information for a specific server.`,
		Args:   cobra.ArbitraryArgs,
		PreRun: func(cmd *cobra.Command, args []string) {},
		RunE: func(cmd *cobra.Command, args []string) error {
			serverDB, err := blinky_utils.ReadServerDB()
			if err != nil {
				return errors.Wrap(err, "failed to read server database")
			}

			if len(args) > 0 {
				for _, server := range args {
					if _, ok := serverDB.Servers[server]; !ok {
						return errors.New("server not found in server database")
					} else {
						if serverDB.DefaultServer == server {
							printServerInfo(server, serverDB.Servers[server], true, "* ")
						} else {
							printServerInfo(server, serverDB.Servers[server], false, "  ")
						}
					}
				}

			} else {
				for name, server := range serverDB.Servers {
					if serverDB.DefaultServer == name {
						printServerInfo(name, server, true, "* ")
					} else {
						printServerInfo(name, server, false, "  ")
					}
				}
			}

			return nil
		},
	}

	return &serverCmd
}

func init() {
	subCmds = append(subCmds, serverCmd())
}
