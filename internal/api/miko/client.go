package miko

import (
	"net/http"

	"github.com/Hayao0819/Kamisato/internal/api/client"
)

type Option = client.Option
type ResponseError = client.ResponseError

func WithHTTPClient(httpClient *http.Client) Option {
	return client.WithHTTPClient(httpClient)
}

func WithReadAttempts(attempts int) Option {
	return client.WithReadAttempts(attempts)
}

type Client struct {
	request *client.Requester
}

func New(base, apiKey string, opts ...Option) (*Client, error) {
	request, err := client.NewAPIKeyRequester(base, apiKey, "miko", opts...)
	if err != nil {
		return nil, err
	}
	return NewWithRequester(request), nil
}

func NewWithRequester(request *client.Requester) *Client {
	return &Client{request: request}
}

type Signer struct {
	request *client.Requester
}

func NewSigner(base, apiKey string, opts ...Option) (*Signer, error) {
	request, err := client.NewAPIKeyRequester(base, apiKey, "signer", opts...)
	if err != nil {
		return nil, err
	}
	return &Signer{request: request}, nil
}
