package client

import (
	"net/http"

	httpclient "github.com/Hayao0819/Kamisato/internal/http/client"
	miko "github.com/Hayao0819/Kamisato/miko/client"
)

type Option = httpclient.Option
type BearerTokenSource = httpclient.BearerTokenSource
type RefreshableBearerTokenSource = httpclient.RefreshableBearerTokenSource
type BearerTokenSourceFunc = httpclient.BearerTokenSourceFunc
type ResponseError = httpclient.ResponseError

func StaticBearer(token string) BearerTokenSource {
	return httpclient.StaticBearer(token)
}

func WithHTTPClient(httpClient *http.Client) Option {
	return httpclient.WithHTTPClient(httpClient)
}

func WithReadAttempts(attempts int) Option {
	return httpclient.WithReadAttempts(attempts)
}

type Client struct {
	*miko.Client
	request *httpclient.Requester
}

func New(base string, source BearerTokenSource, opts ...Option) (*Client, error) {
	request, err := httpclient.NewBearerRequester(base, source, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{Client: miko.NewWithRequester(request), request: request}, nil
}

type Publisher struct {
	request *httpclient.Requester
}

func NewPublisher(base, apiKey string, opts ...Option) (*Publisher, error) {
	request, err := httpclient.NewAPIKeyRequester(base, apiKey, "publisher", opts...)
	if err != nil {
		return nil, err
	}
	return &Publisher{request: request}, nil
}
