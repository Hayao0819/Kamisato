package client

import (
	"net/http"

	httpclient "github.com/Hayao0819/Kamisato/internal/http/client"
)

type Option = httpclient.Option
type ResponseError = httpclient.ResponseError

func WithHTTPClient(httpClient *http.Client) Option {
	return httpclient.WithHTTPClient(httpClient)
}

func WithReadAttempts(attempts int) Option {
	return httpclient.WithReadAttempts(attempts)
}

type Client struct {
	request *httpclient.Requester
}

func New(base, apiKey string, opts ...Option) (*Client, error) {
	request, err := httpclient.NewAPIKeyRequester(base, apiKey, "miko", opts...)
	if err != nil {
		return nil, err
	}
	return NewWithRequester(request), nil
}

func NewWithRequester(request *httpclient.Requester) *Client {
	return &Client{request: request}
}

type Signer struct {
	request *httpclient.Requester
}

func NewSigner(base, apiKey string, opts ...Option) (*Signer, error) {
	request, err := httpclient.NewAPIKeyRequester(base, apiKey, "signer", opts...)
	if err != nil {
		return nil, err
	}
	return &Signer{request: request}, nil
}
