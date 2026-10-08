package bugreport

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type reporterRoundTripFunc func(*http.Request) (*http.Response, error)

func (f reporterRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestReportUsesInjectedHTTPForAllNetworkBackends(t *testing.T) {
	var paths []string
	client := &http.Client{Transport: reporterRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		paths = append(paths, request.URL.Path)
		body := ""
		status := http.StatusNoContent
		if strings.Contains(request.URL.Path, "/issues") {
			status = http.StatusCreated
			body = `{"html_url":"https://github.com/o/r/issues/1"}`
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	reporter, err := New(Config{Backends: []string{"github", "webhook"}, GitHub: GitHubConfig{Repo: "o/r", Token: "secret"}, Webhook: WebhookConfig{URL: "https://tracker.example/hook"}}, client)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reporter.Report(context.Background(), Report{Pkgname: "demo", Description: "broken"}); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[0] != "/repos/o/r/issues" || paths[1] != "/hook" {
		t.Fatalf("tracker requests = %v", paths)
	}
}
