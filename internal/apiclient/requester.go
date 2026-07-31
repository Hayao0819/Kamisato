package apiclient

import (
	"context"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

type Requester struct {
	transport *Transport
	source    BearerTokenSource
}

func NewBearerRequester(base string, source BearerTokenSource, opts ...Option) (*Requester, error) {
	transport, err := newBearerTransport(base, source, opts...)
	if err != nil {
		return nil, err
	}
	if !transport.SecureUserEndpoint() {
		return nil, errors.NewErr("ayato user authentication requires https (http is allowed only for loopback development)")
	}
	return &Requester{transport: transport, source: source}, nil
}

func NewAPIKeyRequester(base, apiKey, name string, opts ...Option) (*Requester, error) {
	if apiKey == "" {
		return nil, errors.NewErr(name + " API key is required")
	}
	transport, err := newAPIKeyTransport(base, apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return &Requester{transport: transport}, nil
}

func (r *Requester) Transport() *Transport {
	return r.transport
}

func (r *Requester) Execute(ctx context.Context, operation func() error) error {
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
