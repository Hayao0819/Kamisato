package ayatoapi

import (
	"net/http"

	"github.com/Hayao0819/Kamisato/internal/apiclient"
	"github.com/Hayao0819/Kamisato/internal/mikoapi"
)

type Option = apiclient.Option
type BearerTokenSource = apiclient.BearerTokenSource
type RefreshableBearerTokenSource = apiclient.RefreshableBearerTokenSource
type BearerTokenSourceFunc = apiclient.BearerTokenSourceFunc
type ResponseError = apiclient.ResponseError

func StaticBearer(token string) BearerTokenSource {
	return apiclient.StaticBearer(token)
}

func WithHTTPClient(client *http.Client) Option {
	return apiclient.WithHTTPClient(client)
}

func WithReadAttempts(attempts int) Option {
	return apiclient.WithReadAttempts(attempts)
}

type Ayato struct {
	*mikoapi.Client
	request *apiclient.Requester
}

func NewAyato(base string, source BearerTokenSource, opts ...Option) (*Ayato, error) {
	request, err := apiclient.NewBearerRequester(base, source, opts...)
	if err != nil {
		return nil, err
	}
	return &Ayato{Client: mikoapi.NewWithRequester(request), request: request}, nil
}

type Publisher struct {
	request *apiclient.Requester
}

func NewPublisher(base, apiKey string, opts ...Option) (*Publisher, error) {
	request, err := apiclient.NewAPIKeyRequester(base, apiKey, "publisher", opts...)
	if err != nil {
		return nil, err
	}
	return &Publisher{request: request}, nil
}
