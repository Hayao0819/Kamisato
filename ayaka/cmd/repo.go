package cmd

import (
	"fmt"

	"github.com/BrenekH/blinky/clientlib"
	blinky_util "github.com/BrenekH/blinky/cmd/blinky/util"
	"github.com/spf13/cobra"
)

// repoCmd groups the commands that publish to and prune the distribution repo
// on ayato. The verbs mirror Arch's repo-add / repo-remove: `repo add` uploads
// built package files, `repo remove` takes them back out.
func repoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Publish packages to the distribution repository on ayato",
		Long:  "Add built packages to, or remove them from, a repository served by ayato.",
	}
	cmd.AddCommand(
		repoAddCmd(),
		repoRemoveCmd(),
	)
	return cmd
}

// repoAddCmd uploads one or more built package files to a repository on ayato.
func repoAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <repo> <package_files...>",
		Short: "Add built packages to a repository on ayato",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := repoClient(cmd)
			if err != nil {
				return err
			}
			repoName := args[0]
			files := args[1:]
			return client.UploadPackageFiles(repoName, files...)
		},
	}
	addRepoServerFlags(cmd)
	return cmd
}

// repoRemoveCmd removes one or more packages from a repository on ayato.
func repoRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <repo> <packages...>",
		Short: "Remove packages from a repository on ayato",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := repoClient(cmd)
			if err != nil {
				return err
			}
			repoName := args[0]
			pkgs := args[1:]
			return client.RemovePackages(repoName, pkgs...)
		},
	}
	addRepoServerFlags(cmd)
	return cmd
}

// addRepoServerFlags registers the server-selection flags shared by repo add
// and repo remove.
func addRepoServerFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("server", "s", "", "ayato server (default: serverdb default)")
	cmd.Flags().String("username", "", "Username for server login (overrides saved value)")
	cmd.Flags().String("password", "", "Password for server login (overrides saved value)")
	cmd.Flags().BoolP("ask-pass", "K", false, "Prompt for password interactively")
}

// repoClient resolves the ayato endpoint and credentials from the flags and the
// server database, then returns a Blinky-compatible client for it.
func repoClient(cmd *cobra.Command) (*clientlib.BlinkyClient, error) {
	serverFlag, err := cmd.Flags().GetString("server")
	if err != nil {
		return nil, err
	}
	usernameFlag, err := cmd.Flags().GetString("username")
	if err != nil {
		return nil, err
	}
	passwordFlag, err := cmd.Flags().GetString("password")
	if err != nil {
		return nil, err
	}
	askPass, err := cmd.Flags().GetBool("ask-pass")
	if err != nil {
		return nil, err
	}

	db, err := blinky_util.ReadServerDB()
	if err != nil {
		return nil, err
	}

	server := serverFlag
	if server == "" {
		server = db.DefaultServer
	}
	if server == "" {
		return nil, fmt.Errorf("no server specified and no default server is set")
	}
	entry, ok := db.Servers[server]
	if !ok {
		return nil, fmt.Errorf("server not found: %s", server)
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
				return nil, err
			}
			password = p
		} else {
			password = entry.Password
		}
	}

	return clientlib.New(server, username, password)
}

func init() {
	subCmds.Add(repoCmd())
}
