package shared

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/client"
)

// AddRepoServerFlags registers the shared --server selection flag plus the
// credential overrides that only the repo-upload path needs.
func AddRepoServerFlags(cmd *cobra.Command) {
	AddServerFlag(cmd)
	cmd.Flags().String("token", "", "Ayato Bearer access token (overrides saved login)")
	cmd.Flags().String("username", "", "Username for server login (overrides saved value)")
	cmd.Flags().String("password", "", "Password for server login (overrides saved value)")
	cmd.Flags().BoolP("ask-pass", "K", false, "Prompt for password interactively")
	_ = cmd.Flags().MarkDeprecated("username", "native Ayato authentication does not use a username")
	_ = cmd.Flags().MarkDeprecated("password", "use --token; Basic authentication is limited to the legacy /blinky API")
	_ = cmd.Flags().MarkDeprecated("ask-pass", "use --token or 'ayaka server login'")
}

// RepoClient resolves an Ayato client with optional credential overrides.
func RepoClient(cmd *cobra.Command) (*client.Ayato, error) {
	server, err := cmd.Flags().GetString("server")
	if err != nil {
		return nil, err
	}
	return RepoClientAt(cmd, server)
}

// RepoClientAt is RepoClient with an explicit server selection, for commands
// whose --server flag keeps a different (legacy) meaning.
func RepoClientAt(cmd *cobra.Command, server string) (*client.Ayato, error) {
	tokenFlag, err := cmd.Flags().GetString("token")
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

	info, err := ResolveAyatoServer(server)
	if err != nil {
		return nil, err
	}

	if tokenFlag != "" {
		info.AccessToken = tokenFlag
	} else if passwordFlag != "" {
		info.AccessToken = passwordFlag
	} else if askPass {
		p, err := PromptPassword("Access token:")
		if err != nil {
			return nil, err
		}
		info.AccessToken = p
	}

	if tokenFlag != "" || passwordFlag != "" || askPass {
		return client.NewAyato(info.URL, client.StaticBearer(info.AccessToken))
	}
	return AyatoClient(info)
}
