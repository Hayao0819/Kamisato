// Package nvcheck monitors upstream sources for new versions of tracked packages
// (an nvchecker-style version watch) and drives a rebuild when a newer version
// appears. It fetches only over the injected *http.Client so callers route every
// outbound call through internal/httpx.
package nvcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/Hayao0819/Kamisato/pkg/pacman/alpm"
)

// VersionSource resolves the latest upstream version of a package.
type VersionSource interface {
	Latest(ctx context.Context) (string, error)
}

// Spec is the config-derived description of one monitored source.
type Spec struct {
	Kind    string
	Repo    string // owner/name for github kinds
	Package string // project name for pypi
	URL     string // fetch target for the http kind
	Regex   string // capture group 1 is the version, for the http kind
	Prefix  string // stripped from the matched version
}

// NewSource builds the VersionSource for spec, fetching through client. The
// default upstream hosts are used; source structs are exported within the package
// so tests can point them at an httptest server.
func NewSource(spec Spec, client *http.Client) (VersionSource, error) {
	if client == nil {
		return nil, fmt.Errorf("nvcheck: nil http client")
	}
	switch spec.Kind {
	case "github":
		if spec.Repo == "" {
			return nil, fmt.Errorf("nvcheck: github source needs a repo")
		}
		return &githubReleaseSource{repo: spec.Repo, prefix: spec.Prefix, base: githubAPIBase, client: client}, nil
	case "github_tag":
		if spec.Repo == "" {
			return nil, fmt.Errorf("nvcheck: github_tag source needs a repo")
		}
		return &githubTagSource{repo: spec.Repo, prefix: spec.Prefix, base: githubAPIBase, client: client}, nil
	case "pypi":
		if spec.Package == "" {
			return nil, fmt.Errorf("nvcheck: pypi source needs a package")
		}
		return &pypiSource{pkg: spec.Package, prefix: spec.Prefix, base: pypiBase, client: client}, nil
	case "http":
		if spec.URL == "" || spec.Regex == "" {
			return nil, fmt.Errorf("nvcheck: http source needs a url and regex")
		}
		re, err := regexp.Compile(spec.Regex)
		if err != nil {
			return nil, fmt.Errorf("nvcheck: invalid regex: %w", err)
		}
		if re.NumSubexp() < 1 {
			return nil, fmt.Errorf("nvcheck: regex must have a capture group for the version")
		}
		return &httpRegexSource{url: spec.URL, re: re, prefix: spec.Prefix, client: client}, nil
	default:
		return nil, fmt.Errorf("nvcheck: unknown source kind %q", spec.Kind)
	}
}

const (
	githubAPIBase = "https://api.github.com"
	pypiBase      = "https://pypi.org"
)

type githubReleaseSource struct {
	repo   string
	prefix string
	base   string
	client *http.Client
}

func (s *githubReleaseSource) Latest(ctx context.Context) (string, error) {
	u := strings.TrimRight(s.base, "/") + "/repos/" + s.repo + "/releases/latest"
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := fetchJSON(ctx, s.client, u, &body); err != nil {
		return "", err
	}
	if body.TagName == "" {
		return "", fmt.Errorf("nvcheck: github release for %s has no tag_name", s.repo)
	}
	return stripPrefix(body.TagName, s.prefix), nil
}

type githubTagSource struct {
	repo   string
	prefix string
	base   string
	client *http.Client
}

func (s *githubTagSource) Latest(ctx context.Context) (string, error) {
	u := strings.TrimRight(s.base, "/") + "/repos/" + s.repo + "/tags"
	var tags []struct {
		Name string `json:"name"`
	}
	if err := fetchJSON(ctx, s.client, u, &tags); err != nil {
		return "", err
	}
	// The tags endpoint is not version-ordered, so pick the highest by vercmp
	// rather than trusting the first element.
	best := ""
	for _, t := range tags {
		v := stripPrefix(t.Name, s.prefix)
		if v == "" {
			continue
		}
		if best == "" {
			best = v
			continue
		}
		if c, _ := alpm.VerCmp(v, best); c > 0 {
			best = v
		}
	}
	if best == "" {
		return "", fmt.Errorf("nvcheck: github tags for %s are empty", s.repo)
	}
	return best, nil
}

type pypiSource struct {
	pkg    string
	prefix string
	base   string
	client *http.Client
}

func (s *pypiSource) Latest(ctx context.Context) (string, error) {
	u := strings.TrimRight(s.base, "/") + "/pypi/" + url.PathEscape(s.pkg) + "/json"
	var body struct {
		Info struct {
			Version string `json:"version"`
		} `json:"info"`
	}
	if err := fetchJSON(ctx, s.client, u, &body); err != nil {
		return "", err
	}
	if body.Info.Version == "" {
		return "", fmt.Errorf("nvcheck: pypi package %s has no version", s.pkg)
	}
	return stripPrefix(body.Info.Version, s.prefix), nil
}

type httpRegexSource struct {
	url    string
	re     *regexp.Regexp
	prefix string
	client *http.Client
}

func (s *httpRegexSource) Latest(ctx context.Context) (string, error) {
	body, err := fetchBody(ctx, s.client, s.url)
	if err != nil {
		return "", err
	}
	m := s.re.FindSubmatch(body)
	if m == nil {
		return "", fmt.Errorf("nvcheck: regex did not match %s", s.url)
	}
	return stripPrefix(string(m[1]), s.prefix), nil
}

func stripPrefix(v, prefix string) string {
	v = strings.TrimSpace(v)
	if prefix != "" {
		v = strings.TrimPrefix(v, prefix)
	}
	return v
}

func fetchJSON(ctx context.Context, client *http.Client, u string, v any) error {
	body, err := fetchBody(ctx, client, u)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("nvcheck: decode %s: %w", u, err)
	}
	return nil
}

// fetchBody GETs u and returns the response body, capping it so a hostile or
// broken endpoint cannot exhaust memory.
func fetchBody(ctx context.Context, client *http.Client, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nvcheck: GET %s: status %d", u, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}
