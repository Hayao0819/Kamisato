package bugreport

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewDisabledAndValidation(t *testing.T) {
	if r, err := New("", "", ""); r != nil || err != nil {
		t.Fatalf("empty type must disable: r=%v err=%v", r, err)
	}
	if _, err := New("github", "noslash", "tok"); err == nil {
		t.Error("github repo without owner/name must error")
	}
	if _, err := New("github", "o/r", ""); err == nil {
		t.Error("github without a token must error")
	}
	if _, err := New("nonsense", "", ""); err == nil {
		t.Error("unknown backend must error")
	}
}

func TestGitHubReport(t *testing.T) {
	var gotPath, gotAuth string
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &payload)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"html_url":"https://github.com/o/r/issues/7"}`))
	}))
	defer srv.Close()

	g := &githubReporter{client: srv.Client(), base: srv.URL, owner: "o", repo: "r", token: "T"}
	url, err := g.Report(context.Background(), Report{
		Pkgname: "foo", Pkgver: "1.0-1", Severity: "high", Name: "n", Email: "e", Description: "boom",
	})
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	if url != "https://github.com/o/r/issues/7" {
		t.Errorf("url = %q", url)
	}
	if gotPath != "/repos/o/r/issues" {
		t.Errorf("path = %q, want /repos/o/r/issues", gotPath)
	}
	if gotAuth != "Bearer T" {
		t.Errorf("auth = %q, want Bearer T", gotAuth)
	}
	title, _ := payload["title"].(string)
	body, _ := payload["body"].(string)
	if !strings.Contains(title, "foo") || !strings.Contains(body, "boom") {
		t.Errorf("issue title=%q body=%q missing package/description", title, body)
	}
}

func TestGitHubReportNon201(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	defer srv.Close()
	g := &githubReporter{client: srv.Client(), base: srv.URL, owner: "o", repo: "r", token: "bad"}
	if _, err := g.Report(context.Background(), Report{Pkgname: "foo", Description: "x"}); err == nil {
		t.Error("a non-201 response must surface an error")
	}
}
