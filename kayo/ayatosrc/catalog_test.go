package ayatosrc

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/api/client"
)

type catalogRoundTripFunc func(*http.Request) (*http.Response, error)

func (f catalogRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestCatalogClientPreservesPrefixAndRetriesReads(t *testing.T) {
	var calls int
	httpClient := &http.Client{Transport: catalogRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.EscapedPath() != "/mirror/root/api/unstable/aur/catalog" {
			t.Fatalf("catalog path = %q", req.URL.EscapedPath())
		}
		if req.Header.Get("Authorization") != "" || req.Header.Get("X-API-Key") != "" {
			t.Fatalf("catalog request carried credentials: %#v", req.Header)
		}
		status := http.StatusOK
		body := `{"payload":"catalog"}`
		if calls == 1 {
			status = http.StatusServiceUnavailable
			body = `{"error":"busy"}`
		}
		return &http.Response{
			StatusCode: status,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})}

	catalog, err := newCatalogClient(
		"https://ayato.example/mirror/root",
		client.WithHTTPClient(httpClient),
		client.WithReadAttempts(2),
	)
	if err != nil {
		t.Fatal(err)
	}
	body, err := catalog.fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || string(body) != `{"payload":"catalog"}` {
		t.Fatalf("calls/body = %d, %q", calls, body)
	}
}
