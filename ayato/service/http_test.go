package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type httpTransport func(*http.Request) (*http.Response, error)

func (f httpTransport) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestGitHubLookupUsesInjectedOutboundClient(t *testing.T) {
	requests := 0
	client := &http.Client{Transport: httpTransport(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.URL.String() != "https://api.github.com/users/example" {
			t.Fatalf("URL = %s", request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"id":123,"login":"Example"}`)), Header: make(http.Header)}, nil
	})}
	svc := New(nil, nil, nil, nil, Settings{}, WithOutboundHTTPClient(client))
	id, login, err := svc.ResolveGitHubLogin(context.Background(), "example")
	if err != nil || id != 123 || login != "Example" || requests != 1 {
		t.Fatalf("lookup = %d, %q, %v; requests = %d", id, login, err, requests)
	}
	if svc.httpClient != client {
		t.Fatal("outbound repositories do not share the injected client")
	}
}
