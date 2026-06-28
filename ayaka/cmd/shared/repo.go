package shared

import (
	"github.com/BrenekH/blinky/clientlib"
	blinky_util "github.com/BrenekH/blinky/cmd/blinky/util"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/spf13/cobra"
)

// AddRepoServerFlags registers the server-selection flags shared by repo add
// and repo remove.
func AddRepoServerFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("server", "s", "", "ayato server (default: serverdb default)")
	cmd.Flags().String("username", "", "Username for server login (overrides saved value)")
	cmd.Flags().String("password", "", "Password for server login (overrides saved value)")
	cmd.Flags().BoolP("ask-pass", "K", false, "Prompt for password interactively")
}

// RepoClient resolves the ayato endpoint and credentials from the flags and the
// server database, then returns a Blinky-compatible client for it.
func RepoClient(cmd *cobra.Command) (*clientlib.BlinkyClient, error) {
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
		return nil, utils.WrapErr(err, "failed to read server database")
	}

	server := serverFlag
	if server == "" {
		server = db.DefaultServer
	}
	if server == "" {
		return nil, ErrNoServerSpecified
	}
	entry, ok := db.Servers[server]
	if !ok {
		return nil, utils.WrapErr(ErrServerNotFound, server)
	}

	username := usernameFlag
	if username == "" {
		username = entry.Username
	}

	password := passwordFlag
	if password == "" {
		if askPass {
			p, err := PromptPassword("Password:")
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
