package ayato

import (
	"net/http"

	"github.com/Hayao0819/Kamisato/internal/api/client"
	"github.com/Hayao0819/Kamisato/internal/api/miko"
)

type Option = client.Option
type BearerTokenSource = client.BearerTokenSource
type RefreshableBearerTokenSource = client.RefreshableBearerTokenSource
type BearerTokenSourceFunc = client.BearerTokenSourceFunc
type ResponseError = client.ResponseError

func StaticBearer(token string) BearerTokenSource {
	return client.StaticBearer(token)
}

func WithHTTPClient(httpClient *http.Client) Option {
	return client.WithHTTPClient(httpClient)
}

func WithReadAttempts(attempts int) Option {
	return client.WithReadAttempts(attempts)
}

type Client struct {
	*miko.Client
	request *client.Requester
}

func New(base string, source BearerTokenSource, opts ...Option) (*Client, error) {
	request, err := client.NewBearerRequester(base, source, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{Client: miko.NewWithRequester(request), request: request}, nil
}

type Publisher struct {
	request *client.Requester
}

func NewPublisher(base, apiKey string, opts ...Option) (*Publisher, error) {
	request, err := client.NewAPIKeyRequester(base, apiKey, "publisher", opts...)
	if err != nil {
		return nil, err
	}
	return &Publisher{request: request}, nil
}
