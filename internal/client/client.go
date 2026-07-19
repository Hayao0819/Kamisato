package client

import (
	"context"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

type requester struct {
	transport *transport
	source    BearerTokenSource
}

// execute retries an operation once after token refresh.
func (r *requester) execute(ctx context.Context, operation func() error) error {
	refreshable, canRefresh := r.source.(RefreshableBearerTokenSource)
	staleToken := ""
	if canRefresh {
		var err error
		staleToken, err = refreshable.Token(ctx)
		if err != nil {
			return errors.WrapErr(err, "resolve request credential")
		}
	}
	err := operation()
	if !errors.Is(err, ErrAccessTokenExpired) {
		return err
	}
	if !canRefresh {
		return err
	}
	if refreshErr := refreshable.RefreshIfCurrent(ctx, staleToken); refreshErr != nil {
		return errors.WrapErr(refreshErr, "session expired; please run 'ayaka server login' again")
	}
	return operation()
}

type BuildClient struct{ request *requester }

// Ayato is the user-facing Ayato client.
type Ayato struct {
	*BuildClient
	request *requester
}

// Miko is the direct build-server client.
type Miko struct{ *BuildClient }

type apiKeyClient struct{ request *requester }

// Publisher exposes package publication and worker registration.
type Publisher apiKeyClient

// Signer exposes remote detached signing.
type Signer apiKeyClient

func NewAyato(base string, source BearerTokenSource, opts ...Option) (*Ayato, error) {
	t, err := newTransport(base, bearerCredential{source: source}, opts...)
	if err != nil {
		return nil, err
	}
	if !secureUserEndpoint(t.base) {
		return nil, errors.NewErr("ayato user authentication requires https (http is allowed only for loopback development)")
	}
	r := &requester{transport: t, source: source}
	return &Ayato{BuildClient: &BuildClient{request: r}, request: r}, nil
}

func NewMiko(base, apiKey string, opts ...Option) (*Miko, error) {
	client, err := newAPIKeyClient(base, apiKey, "miko", opts...)
	if err != nil {
		return nil, err
	}
	return &Miko{BuildClient: &BuildClient{request: client.request}}, nil
}

func NewPublisher(base, apiKey string, opts ...Option) (*Publisher, error) {
	client, err := newAPIKeyClient(base, apiKey, "publisher", opts...)
	return (*Publisher)(client), err
}

func NewSigner(base, apiKey string, opts ...Option) (*Signer, error) {
	client, err := newAPIKeyClient(base, apiKey, "signer", opts...)
	return (*Signer)(client), err
}

func newAPIKeyClient(base, apiKey, name string, opts ...Option) (*apiKeyClient, error) {
	if apiKey == "" {
		return nil, errors.NewErr(name + " API key is required")
	}
	t, err := newTransport(base, apiKeyCredential{key: apiKey}, opts...)
	if err != nil {
		return nil, err
	}
	return &apiKeyClient{request: &requester{transport: t}}, nil
}
