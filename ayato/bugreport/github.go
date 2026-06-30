package bugreport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const githubAPI = "https://api.github.com"

// githubReporter opens a GitHub issue per report. It uses the REST API directly
// (no SDK dependency) with a fine-grained or classic token that can write issues.
type githubReporter struct {
	client *http.Client
	base   string
	owner  string
	repo   string
	token  string
}

func newGitHub(cfg GitHubConfig) (Reporter, error) {
	owner, name, ok := strings.Cut(cfg.Repo, "/")
	if !ok || owner == "" || name == "" {
		return nil, fmt.Errorf("bugreport: github repo must be \"owner/name\", got %q", cfg.Repo)
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("bugreport: github token is required")
	}
	return &githubReporter{client: &http.Client{Timeout: 15 * time.Second}, base: githubAPI, owner: owner, repo: name, token: cfg.Token}, nil
}

func (g *githubReporter) Report(ctx context.Context, r Report) (string, error) {
	payload := map[string]any{
		"title": issueTitle(r),
		"body":  issueBody(r),
	}
	if r.Severity != "" {
		payload["labels"] = []string{"bug", r.Severity}
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/repos/%s/%s/issues", g.base, g.owner, g.repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("bugreport: github request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<10))
		return "", fmt.Errorf("bugreport: github issue creation failed (%s): %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var out struct {
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.HTMLURL, nil
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
