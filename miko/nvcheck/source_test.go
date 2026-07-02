package nvcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGithubReleaseSource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/proj/releases/latest" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"tag_name":"v2.5.1","name":"release"}`))
	}))
	defer srv.Close()

	src := &githubReleaseSource{repo: "owner/proj", prefix: "v", base: srv.URL, client: srv.Client()}
	got, err := src.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got != "2.5.1" {
		t.Errorf("got %q, want 2.5.1", got)
	}
}

func TestGithubTagSourcePicksHighest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/proj/tags" {
			http.NotFound(w, r)
			return
		}
		// Deliberately unordered: the source must pick the highest by vercmp.
		_, _ = w.Write([]byte(`[{"name":"v1.2.0"},{"name":"v1.10.0"},{"name":"v1.9.0"}]`))
	}))
	defer srv.Close()

	src := &githubTagSource{repo: "owner/proj", prefix: "v", base: srv.URL, client: srv.Client()}
	got, err := src.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got != "1.10.0" {
		t.Errorf("got %q, want 1.10.0", got)
	}
}

func TestPypiSource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pypi/requests/json" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"info":{"version":"2.31.0"}}`))
	}))
	defer srv.Close()

	src := &pypiSource{pkg: "requests", base: srv.URL, client: srv.Client()}
	got, err := src.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got != "2.31.0" {
		t.Errorf("got %q, want 2.31.0", got)
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
