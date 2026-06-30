package shared

import (
	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/spf13/cobra"
)

// AddRepoServerFlags registers the shared server-selection flags.
func AddRepoServerFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("server", "s", "", "ayato server (default: serverdb default)")
	cmd.Flags().String("username", "", "Username for server login (overrides saved value)")
	cmd.Flags().String("password", "", "Password for server login (overrides saved value)")
	cmd.Flags().BoolP("ask-pass", "K", false, "Prompt for password interactively")
}

// RepoClient resolves the endpoint and credentials from flags and the serverdb, returning a Blinky client.
func RepoClient(cmd *cobra.Command) (*blinkyutils.Client, error) {
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

	info, err := blinkyutils.ResolveServer(serverFlag)
	if err != nil {
		return nil, err
	}

	if usernameFlag != "" {
		info.Username = usernameFlag
	}
	if passwordFlag != "" {
		info.Password = passwordFlag
	} else if askPass {
		p, err := PromptPassword("Password:")
		if err != nil {
			return nil, err
		}
		info.Password = p
	}

	return info.Client()
}
