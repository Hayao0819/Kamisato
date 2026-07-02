package bugreport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-github/v88/github"
)

func TestNewDisabledAndValidation(t *testing.T) {
	if r, err := New(Config{}); r != nil || err != nil {
		t.Fatalf("no backends must disable: r=%v err=%v", r, err)
	}
	if _, err := New(Config{Backends: []string{"github"}, GitHub: GitHubConfig{Repo: "noslash", Token: "tok"}}); err == nil {
		t.Error("github repo without owner/name must error")
	}
	if _, err := New(Config{Backends: []string{"github"}, GitHub: GitHubConfig{Repo: "o/r"}}); err == nil {
		t.Error("github without a token must error")
	}
	if _, err := New(Config{Backends: []string{"nonsense"}}); err == nil {
		t.Error("unknown backend must error")
	}
}

// fakeIssues records the issue it is asked to create.
type fakeIssues struct {
	got issue
	url string
	err error
}

func (f *fakeIssues) CreateIssue(_ context.Context, in issue) (string, error) {
	f.got = in
	return f.url, f.err
}

func TestGitHubReport(t *testing.T) {
	fake := &fakeIssues{url: "https://github.com/o/r/issues/7"}
	g := &githubReporter{issues: fake}
	url, err := g.Report(context.Background(), Report{
		Pkgname: "foo", Pkgver: "1.0-1", Severity: "high", Name: "n", Email: "e", Description: "boom",
	})
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	if url != "https://github.com/o/r/issues/7" {
		t.Errorf("url = %q", url)
	}
	if !strings.Contains(fake.got.Title, "foo") || !strings.Contains(fake.got.Body, "boom") {
		t.Errorf("issue title=%q body=%q missing package/description", fake.got.Title, fake.got.Body)
	}
	if len(fake.got.Labels) != 2 || fake.got.Labels[0] != "bug" || fake.got.Labels[1] != "high" {
		t.Errorf("labels = %v, want [bug high]", fake.got.Labels)
	}
}

func TestGitHubReportError(t *testing.T) {
	g := &githubReporter{issues: &fakeIssues{err: errors.New("boom")}}
	if _, err := g.Report(context.Background(), Report{Pkgname: "foo", Description: "x"}); err == nil {
		t.Error("a creator error must surface")
	}
}

func TestGHAPICreateIssue(t *testing.T) {
	var gotPath, gotAuth string
	var body struct {
		Title  string   `json:"title"`
		Body   string   `json:"body"`
		Labels []string `json:"labels"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"html_url":"https://github.com/o/r/issues/7"}`))
	}))
	defer srv.Close()

	c, err := github.NewClient(github.WithAuthToken("T"), github.WithEnterpriseURLs(srv.URL, srv.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	a := ghAPI{c: c, owner: "o", repo: "r"}
	url, err := a.CreateIssue(context.Background(), issue{Title: "t", Body: "b", Labels: []string{"bug", "high"}})
	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	if url != "https://github.com/o/r/issues/7" {
		t.Errorf("url = %q", url)
	}
	if !strings.HasSuffix(gotPath, "/repos/o/r/issues") {
		t.Errorf("path = %q, want suffix /repos/o/r/issues", gotPath)
	}
	if gotAuth != "Bearer T" {
		t.Errorf("auth = %q, want Bearer T", gotAuth)
	}
	if body.Title != "t" || body.Body != "b" || len(body.Labels) != 2 {
		t.Errorf("posted body = %+v", body)
	}
}

func TestGHAPICreateIssueError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	defer srv.Close()

	c, err := github.NewClient(github.WithAuthToken("bad"), github.WithEnterpriseURLs(srv.URL, srv.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	a := ghAPI{c: c, owner: "o", repo: "r"}
	if _, err := a.CreateIssue(context.Background(), issue{Title: "t", Body: "b"}); err == nil {
		t.Error("a non-201 response must surface an error")
	}
}
