package remote

import (
	ayato "github.com/Hayao0819/Kamisato/ayato/client"
	"github.com/Hayao0819/Kamisato/ayato/client/auth/store"
	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
)

// Provider owns endpoint resolution and client construction, not command state.
// Supplying these operations also supplies their HTTP and credential storage.
type Provider struct {
	Resolve      func(string) (*AyatoServer, error)
	StoredClient func(*AyatoServer, ...ayato.Option) (*ayato.Client, error)
	BearerClient func(string, string) (*ayato.Client, error)
	Prompt       func() (string, error)
}

func DefaultProvider() Provider {
	return Provider{
		Resolve:      store.Resolve,
		StoredClient: store.NewStoredClient,
		BearerClient: func(endpoint, token string) (*ayato.Client, error) {
			return ayato.New(endpoint, ayato.StaticBearer(token))
		},
		Prompt: func() (string, error) { return cmdline.PromptPassword("Access token:") },
	}
}
