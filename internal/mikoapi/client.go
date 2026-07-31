package mikoapi

import (
	"net/http"

	"github.com/Hayao0819/Kamisato/internal/apiclient"
)

type Option = apiclient.Option
type ResponseError = apiclient.ResponseError

func WithHTTPClient(client *http.Client) Option {
	return apiclient.WithHTTPClient(client)
}

func WithReadAttempts(attempts int) Option {
	return apiclient.WithReadAttempts(attempts)
}

type Client struct {
	request *apiclient.Requester
}

func New(base, apiKey string, opts ...Option) (*Client, error) {
	request, err := apiclient.NewAPIKeyRequester(base, apiKey, "miko", opts...)
	if err != nil {
		return nil, err
	}
	return NewWithRequester(request), nil
}

func NewWithRequester(request *apiclient.Requester) *Client {
	return &Client{request: request}
}

type Signer struct {
	request *apiclient.Requester
}

func NewSigner(base, apiKey string, opts ...Option) (*Signer, error) {
	request, err := apiclient.NewAPIKeyRequester(base, apiKey, "signer", opts...)
	if err != nil {
		return nil, err
	}
	return &Signer{request: request}, nil
}
