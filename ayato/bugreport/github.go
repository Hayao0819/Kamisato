package bugreport

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v88/github"
)

// issue is the tracker-agnostic shape of an issue to open; business logic builds
// it from a Report and never touches the GitHub client.
type issue struct {
	Title  string
	Body   string
	Labels []string
}

// issueCreator is the narrow port the reporter depends on, so tests can inject a
// fake without an HTTP round-trip.
type issueCreator interface {
	CreateIssue(ctx context.Context, in issue) (url string, err error)
}

// githubReporter opens a GitHub issue per report through an issueCreator.
type githubReporter struct {
	issues issueCreator
}

func newGitHub(cfg GitHubConfig) (Reporter, error) {
	owner, name, ok := strings.Cut(cfg.Repo, "/")
	if !ok || owner == "" || name == "" {
		return nil, fmt.Errorf("bugreport: github repo must be \"owner/name\", got %q", cfg.Repo)
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("bugreport: github token is required")
	}
	c, err := github.NewClient(github.WithAuthToken(cfg.Token))
	if err != nil {
		return nil, fmt.Errorf("bugreport: github client: %w", err)
	}
	return &githubReporter{issues: ghAPI{c: c, owner: owner, repo: name}}, nil
}

func (g *githubReporter) Report(ctx context.Context, r Report) (string, error) {
	return g.issues.CreateIssue(ctx, toIssue(r))
}

// ghAPI adapts the go-github client to issueCreator.
type ghAPI struct {
	c     *github.Client
	owner string
	repo  string
}

func (a ghAPI) CreateIssue(ctx context.Context, in issue) (string, error) {
	iss, _, err := a.c.Issues.Create(ctx, a.owner, a.repo, &github.IssueRequest{
		Title:  &in.Title,
		Body:   &in.Body,
		Labels: &in.Labels,
	})
	if err != nil {
		return "", fmt.Errorf("bugreport: github issue creation failed: %w", err)
	}
	return iss.GetHTMLURL(), nil
}

func toIssue(r Report) issue {
	in := issue{Title: issueTitle(r), Body: issueBody(r)}
	if r.Severity != "" {
		in.Labels = []string{"bug", r.Severity}
	}
	return in
}

func issueTitle(r Report) string {
	pkg := r.Pkgname
	if r.Pkgver != "" {
		pkg += " " + r.Pkgver
	}
	return fmt.Sprintf("[bug] %s", pkg)
}

func issueBody(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**Package:** %s\n", r.Pkgname)
	if r.Pkgver != "" {
		fmt.Fprintf(&b, "**Version:** %s\n", r.Pkgver)
	}
	if r.Severity != "" {
		fmt.Fprintf(&b, "**Severity:** %s\n", r.Severity)
	}
	if r.Name != "" || r.Email != "" {
		fmt.Fprintf(&b, "**Reporter:** %s %s\n", r.Name, r.Email)
	}
	b.WriteString("\n")
	b.WriteString(r.Description)
	return b.String()
}
