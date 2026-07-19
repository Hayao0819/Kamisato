package nvcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func assertLatest(
	t *testing.T,
	path, body, want string,
	newSource func(base string, client *http.Client) VersionSource,
) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	src := newSource(srv.URL, srv.Client())
	got, err := src.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestJSONSources(t *testing.T) {
	tests := []struct {
		name, path, body, want string
		newSource              func(string, *http.Client) VersionSource
	}{
		{
			"GitHub release", "/repos/owner/proj/releases/latest",
			`{"tag_name":"v2.5.1","name":"release"}`, "2.5.1",
			func(base string, client *http.Client) VersionSource {
				return &githubReleaseSource{repo: "owner/proj", prefix: "v", base: base, client: client}
			},
		},
		{
			"GitHub tags", "/repos/owner/proj/tags",
			// Deliberately unordered: the source must pick the highest by vercmp.
			`[{"name":"v1.2.0"},{"name":"v1.10.0"},{"name":"v1.9.0"}]`, "1.10.0",
			func(base string, client *http.Client) VersionSource {
				return &githubTagSource{repo: "owner/proj", prefix: "v", base: base, client: client}
			},
		},
		{
			"PyPI", "/pypi/requests/json", `{"info":{"version":"2.31.0"}}`, "2.31.0",
			func(base string, client *http.Client) VersionSource {
				return &pypiSource{pkg: "requests", base: base, client: client}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertLatest(t, tt.path, tt.body, tt.want, tt.newSource)
		})
	}
}

func TestHTTPRegexSource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body>Latest release: foo-3.4.5.tar.gz</body></html>`))
	}))
	defer srv.Close()

	spec := Spec{Kind: "http", URL: srv.URL, Regex: `foo-([0-9.]+)\.tar\.gz`}
	src, err := NewSource(spec, srv.Client())
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	got, err := src.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got != "3.4.5" {
		t.Errorf("got %q, want 3.4.5", got)
	}
}

func TestNewSourceValidation(t *testing.T) {
	client := http.DefaultClient
	cases := []Spec{
		{Kind: "github"},             // missing repo
		{Kind: "github_tag"},         // missing repo
		{Kind: "pypi"},               // missing package
		{Kind: "http", URL: "x"},     // missing regex
		{Kind: "http", Regex: "(x)"}, // missing url
		{Kind: "http", URL: "u", Regex: "no-group"},
		{Kind: "bogus"},
	}
	for _, spec := range cases {
		if _, err := NewSource(spec, client); err == nil {
			t.Errorf("NewSource(%+v) = nil error, want error", spec)
		}
	}
}

func TestSourceHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	src := &pypiSource{pkg: "x", base: srv.URL, client: srv.Client()}
	if _, err := src.Latest(context.Background()); err == nil {
		t.Error("expected an error on a 500 response")
	}
}
