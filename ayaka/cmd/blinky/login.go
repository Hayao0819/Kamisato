package blinkycmd

import (
	"fmt"
	"strings"

	blinky_util "github.com/BrenekH/blinky/cmd/blinky/util"
	"github.com/spf13/cobra"
)

func loginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login <server_url>",
		Short: "Save login info for a Blinky server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			serverURL := args[0]

			setDefault, err := cmd.Flags().GetBool("default")
			if err != nil {
				return err
			}
			usernameFlag, err := cmd.Flags().GetString("username")
			if err != nil {
				return err
			}
			passwordFlag, err := cmd.Flags().GetString("password")
			if err != nil {
				return err
			}

			db, err := blinky_util.ReadServerDB()
			if err != nil {
				return err
			}

			entry, exists := db.Servers[serverURL]
			username := usernameFlag
			password := passwordFlag

			if exists {
				fmt.Fprintf(cmd.OutOrStdout(), "Login information already exists for %s\n", serverURL)
				answer, err := promptInput("Override? (y/N):")
				if err != nil {
					return err
				}
				if strings.ToLower(strings.TrimSpace(answer)) != "y" {
					// keep existing entry as-is
					if setDefault {
						db.DefaultServer = serverURL
					}
					return blinky_util.SaveServerDB(db)
				}
			}

			if username == "" {
				u, err := promptInput("Username:")
				if err != nil {
					return err
				}
				username = u
			}
			if password == "" {
				p, err := promptPassword("Password:")
				if err != nil {
					return err
				}
				password = p
			}

			entry.Username = username
			entry.Password = password
			db.Servers[serverURL] = entry

			if setDefault {
				db.DefaultServer = serverURL
			}

			return blinky_util.SaveServerDB(db)
		},
	}

	cmd.Flags().Bool("default", false, "Set server as default for upload/remove")
	cmd.Flags().String("username", "", "Username for server login")
	cmd.Flags().String("password", "", "Password for server login (interactive prompt recommended)")

	return cmd
}

func init() {
	subCmds = append(subCmds, loginCmd())
}
