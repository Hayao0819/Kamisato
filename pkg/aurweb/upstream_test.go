package aurweb

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAURUpstreamRetriesNetworkError(t *testing.T) {
	attempts := 0
	client := &http.Client{Transport: upstreamRoundTripFunc(func(*http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("temporary network error")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"results":[{"Name":"demo","PackageBase":"demo","Version":"1-1"}]}`)),
		}, nil
	})}

	packages, err := NewAURUpstream("https://aur.example/rpc", WithHTTPClient(client)).Info(t.Context(), []string{"demo"})
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if attempts != 2 || len(packages) != 1 || packages[0].Name != "demo" {
		t.Fatalf("attempts = %d, packages = %+v", attempts, packages)
	}
}

func TestAURUpstreamRetriesServerError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "temporary", http.StatusBadGateway)
			return
		}
		_, _ = io.WriteString(w, `{"results":[{"Name":"demo","PackageBase":"demo","Version":"1-1"}]}`)
	}))
	defer server.Close()

	packages, err := NewAURUpstream(server.URL).Info(t.Context(), []string{"demo"})
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if attempts != 2 || len(packages) != 1 {
		t.Fatalf("attempts = %d, packages = %+v", attempts, packages)
	}
}

type upstreamRoundTripFunc func(*http.Request) (*http.Response, error)

func (f upstreamRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRetryAfterCapsBeforeDurationConversion(t *testing.T) {
	resp := &http.Response{Header: http.Header{"Retry-After": []string{"9223372036854775807"}}}
	if got := retryAfter(resp, 0); got != 30*time.Second {
		t.Fatalf("retry delay = %s, want 30s", got)
	}
}
