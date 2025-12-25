package blinkycmd

import (
	"fmt"

	"github.com/BrenekH/blinky/clientlib"
	blinky_util "github.com/BrenekH/blinky/cmd/blinky/util"
	"github.com/spf13/cobra"
)

// uploadCmd uploads one or more package files to a Blinky server.
// Usage: ayaka blinky upload [--server URL] [--username USER] [--password PASS] [--ask-pass] <repo> <pkg...>
func uploadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload <repo_name> <package_files...>",
		Short: "Upload packages to a Blinky server",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			serverFlag, err := cmd.Flags().GetString("server")
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
			askPass, err := cmd.Flags().GetBool("ask-pass")
			if err != nil {
				return err
			}

			db, err := blinky_util.ReadServerDB()
			if err != nil {
				return err
			}

			server := serverFlag
			if server == "" {
				server = db.DefaultServer
			}
			entry, ok := db.Servers[server]
			if !ok {
				return fmt.Errorf("server not found: %s", server)
			}

			username := usernameFlag
			if username == "" {
				username = entry.Username
			}

			password := passwordFlag
			if password == "" {
				if askPass {
					p, err := promptPassword("Password:")
					if err != nil {
						return err
					}
					password = p
				} else {
					password = entry.Password
				}
			}

			client, err := clientlib.New(server, username, password)
			if err != nil {
				return err
			}

			repoName := args[0]
			files := args[1:]
			if err := client.UploadPackageFiles(repoName, files...); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringP("server", "s", "", "Server URL to upload to (default: serverdb default)")
	cmd.Flags().String("username", "", "Username for server login (overrides saved value)")
	cmd.Flags().String("password", "", "Password for server login (overrides saved value)")
	cmd.Flags().BoolP("ask-pass", "K", false, "Prompt for password interactively")

	return cmd
}

func init() {
	subCmds = append(subCmds, uploadCmd())
}
