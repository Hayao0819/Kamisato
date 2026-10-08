package remote

import (
	"context"
	"sync"

	ayato "github.com/Hayao0819/Kamisato/ayato/client"
	"github.com/spf13/cobra"
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
func (p Provider) RepoClient(cmd *cobra.Command) (*ayato.Client, error) {
	server, err := cmd.Flags().GetString("server")
	if err != nil {
		return nil, err
	}
	options, err := ReadUpload(cmd)
	if err != nil {
		return nil, err
	}
	info, err := p.Resolve(server)
	if err != nil {
		return nil, err
	}
	return p.Client(info, options)
}

type UploadOptions struct {
	Token    string
	Password string
	AskPass  bool
}

func ReadUpload(cmd *cobra.Command) (UploadOptions, error) {
	tokenFlag, err := cmd.Flags().GetString("token")
	if err != nil {
		return UploadOptions{}, err
	}
	passwordFlag, err := cmd.Flags().GetString("password")
	if err != nil {
		return UploadOptions{}, err
	}
	askPass, err := cmd.Flags().GetBool("ask-pass")
	if err != nil {
		return UploadOptions{}, err
	}
	return UploadOptions{Token: tokenFlag, Password: passwordFlag, AskPass: askPass}, nil
}

// Client applies overrides without mutating the resolved credential source.
func (p Provider) Client(endpoint *AyatoServer, o UploadOptions) (*ayato.Client, error) {
	info := *endpoint
	if o.Token != "" {
		info.AccessToken = o.Token
	} else if o.Password != "" {
		info.AccessToken = o.Password
	} else if o.AskPass {
		token, err := p.Prompt()
		if err != nil {
			return nil, err
		}
		info.AccessToken = token
	}

	if o.Token != "" || o.Password != "" || o.AskPass {
		return p.BearerClient(info.URL, info.AccessToken)
	}
	return p.StoredClient(&info)
}

// Removal decodes flags now but resolves credentials only on the first removal.
// The returned operation retains values, not a mutable Cobra command. Dry runs
// consequently do not prompt or require a configured server.
func (p Provider) Removal(cmd *cobra.Command) (func(context.Context, string, string) error, error) {
	server, err := cmd.Flags().GetString("server")
	if err != nil {
		return nil, err
	}
	options, err := ReadUpload(cmd)
	if err != nil {
		return nil, err
	}
	load := sync.OnceValues(func() (*ayato.Client, error) {
		endpoint, err := p.Resolve(server)
		if err != nil {
			return nil, err
		}
		return p.Client(endpoint, options)
	})
	return func(ctx context.Context, repository, name string) error {
		client, err := load()
		if err != nil {
			return err
		}
		return client.RemovePackageAllArchitectures(ctx, repository, name)
	}, nil
}
