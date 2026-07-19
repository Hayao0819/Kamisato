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

// Publisher exposes package publication and worker registration.
type Publisher struct{ request *requester }

// Signer exposes remote detached signing.
type Signer struct{ request *requester }

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
	if apiKey == "" {
		return nil, errors.NewErr("miko API key is required")
	}
	t, err := newTransport(base, apiKeyCredential{key: apiKey}, opts...)
	if err != nil {
		return nil, err
	}
	return &Miko{BuildClient: &BuildClient{request: &requester{transport: t}}}, nil
}

func NewPublisher(base, apiKey string, opts ...Option) (*Publisher, error) {
	if apiKey == "" {
		return nil, errors.NewErr("publisher API key is required")
	}
	t, err := newTransport(base, apiKeyCredential{key: apiKey}, opts...)
	if err != nil {
		return nil, err
	}
	return &Publisher{request: &requester{transport: t}}, nil
}

func NewSigner(base, apiKey string, opts ...Option) (*Signer, error) {
	if apiKey == "" {
		return nil, errors.NewErr("signer API key is required")
	}
	t, err := newTransport(base, apiKeyCredential{key: apiKey}, opts...)
	if err != nil {
		return nil, err
	}
	return &Signer{request: &requester{transport: t}}, nil
}
