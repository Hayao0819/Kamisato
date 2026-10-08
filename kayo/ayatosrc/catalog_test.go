package ayatosrc

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	httpclient "github.com/Hayao0819/Kamisato/internal/http/client"
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
		httpclient.WithHTTPClient(httpClient),
		httpclient.WithReadAttempts(2),
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

func TestPublicSourceAndKeyLookupUseInjectedTransport(t *testing.T) {
	client := &http.Client{Transport: catalogRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Authorization") != "" || req.Header.Get("X-API-Key") != "" {
			t.Fatal("public source leaked credentials")
		}
		var body string
		switch req.URL.Path {
		case "/api/unstable/aur/catalog":
			body = `{"packages":[{"Name":"example","PackageBase":"example"}]}`
		case "/api/unstable/aur/pubkey":
			body = `{"pubkey":"test-key","key_id":"test-id"}`
		default:
			t.Fatalf("path=%s", req.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	source, err := New(Options{Name: "test", BaseURL: "https://example.invalid", Insecure: true}, httpclient.WithHTTPClient(client))
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	pkgs, err := source.Info(context.Background(), []string{"example"})
	if err != nil || len(pkgs) != 1 {
		t.Fatalf("packages=%+v, error=%v", pkgs, err)
	}
	pub, id, err := FetchPubkey(context.Background(), "https://example.invalid", httpclient.WithHTTPClient(client))
	if err != nil || pub != "test-key" || id != "test-id" {
		t.Fatalf("key/id/error=%q/%q/%v", pub, id, err)
	}
}
