package shared

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
)

// AddRepoServerFlags registers the shared --server selection flag plus the
// credential overrides that only the repo-upload path needs.
func AddRepoServerFlags(cmd *cobra.Command) {
	AddServerFlag(cmd)
	cmd.Flags().String("username", "", "Username for server login (overrides saved value)")
	cmd.Flags().String("password", "", "Password for server login (overrides saved value)")
	cmd.Flags().BoolP("ask-pass", "K", false, "Prompt for password interactively")
}

// RepoClient resolves the endpoint and credentials from flags and the serverdb, returning a Blinky client.
func RepoClient(cmd *cobra.Command) (*blinkyutils.Client, error) {
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

	info, err := ServerFromFlag(cmd)
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
