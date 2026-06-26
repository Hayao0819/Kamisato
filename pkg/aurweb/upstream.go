package aurweb

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultUserAgent is sent on every upstream request; the AUR blocks the default
// user agents of common HTTP libraries.
const DefaultUserAgent = "kamisato-aurweb/1.0 (+https://github.com/Hayao0819/Kamisato)"

const DefaultAURBase = "https://aur.archlinux.org"

// upstreamBatchSize bounds how many pkgnames go into one info GET, matching the
// AUR helper convention and keeping the URL within limits.
const upstreamBatchSize = 150

// AURUpstream calls a real aurweb instance's /rpc endpoint to satisfy packages
// the local Backend does not manage. It implements Upstream.
type AURUpstream struct {
	rpcURL     string
	gitBase    string
	userAgent  string
	client     *http.Client
	dumpClient *http.Client
}

type AURUpstreamOption func(*AURUpstream)

// WithHTTPClient overrides the HTTP client (e.g. to inject timeouts or a test
// transport).
func WithHTTPClient(c *http.Client) AURUpstreamOption {
	return func(u *AURUpstream) { u.client = c }
}

// WithUserAgent overrides the request User-Agent.
func WithUserAgent(ua string) AURUpstreamOption {
	return func(u *AURUpstream) {
		if ua != "" {
			u.userAgent = ua
		}
	}
}

// WithGitBase overrides the git clone base used for redirects (defaults to the
// origin of rpcURL).
func WithGitBase(base string) AURUpstreamOption {
	return func(u *AURUpstream) {
		if base != "" {
			u.gitBase = strings.TrimRight(base, "/")
		}
	}
}

// NewAURUpstream builds an upstream client. rpcURL is the /rpc endpoint, e.g.
// "https://aur.archlinux.org/rpc"; an empty value uses the canonical AUR.
func NewAURUpstream(rpcURL string, opts ...AURUpstreamOption) *AURUpstream {
	if rpcURL == "" {
		rpcURL = DefaultAURBase + "/rpc"
	}
	rpcURL = strings.TrimRight(rpcURL, "/")
	rpcURL = strings.TrimSuffix(rpcURL, "?")

	u := &AURUpstream{
		rpcURL:     rpcURL,
		gitBase:    deriveOrigin(rpcURL),
		userAgent:  DefaultUserAgent,
		client:     &http.Client{Timeout: 15 * time.Second},
		dumpClient: &http.Client{Timeout: 3 * time.Minute},
	}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

func (u *AURUpstream) GitBase() string { return u.gitBase }

// maxNamesBytes bounds the decompressed packages.gz read.
const maxNamesBytes = 64 << 20

func (u *AURUpstream) DumpReader(ctx context.Context, ext bool) (io.ReadCloser, error) {
	name := "/packages-meta-ext-v1.json.gz"
	if !ext {
		name = "/packages-meta-v1.json.gz"
	}
	return u.gzipStream(ctx, u.gitBase+name)
}

func (u *AURUpstream) FetchNames(ctx context.Context) ([]string, error) {
	rc, err := u.gzipStream(ctx, u.gitBase+"/packages.gz")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(io.LimitReader(rc, maxNamesBytes))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out, nil
}

// gzipStream GETs a .gz URL and returns its decompressed body; closing the
// result closes the underlying response.
func (u *AURUpstream) gzipStream(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", u.userAgent)

	resp, err := u.dumpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("aurweb: upstream dump request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("aurweb: upstream dump status %d", resp.StatusCode)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("aurweb: upstream dump gunzip: %w", err)
	}
	return gzipReadCloser{gz: gz, body: resp.Body}, nil
}

type gzipReadCloser struct {
	gz   *gzip.Reader
	body io.ReadCloser
}

func (g gzipReadCloser) Read(p []byte) (int, error) { return g.gz.Read(p) }

func (g gzipReadCloser) Close() error {
	err := g.gz.Close()
	if berr := g.body.Close(); err == nil {
		err = berr
	}
	return err
}

func (u *AURUpstream) SnapshotURL(pkgbase string) string {
	return u.gitBase + "/cgit/aur.git/snapshot/" + url.PathEscape(pkgbase) + ".tar.gz"
}

func (u *AURUpstream) PlainURL(pkgname string) string {
	return u.gitBase + "/cgit/aur.git/plain/PKGBUILD?h=" + url.QueryEscape(pkgname)
}

func (u *AURUpstream) Info(ctx context.Context, names []string) ([]Pkg, error) {
	var out []Pkg
	for chunk := range slicesChunk(names, upstreamBatchSize) {
		v := url.Values{}
		v.Set("v", strconv.Itoa(Version))
		v.Set("type", "info")
		for _, n := range chunk {
			v.Add("arg[]", n)
		}
		res, err := u.do(ctx, v)
		if err != nil {
			return nil, err
		}
		out = append(out, res...)
	}
	return out, nil
}

func (u *AURUpstream) Search(ctx context.Context, by By, arg string) ([]Pkg, error) {
	v := url.Values{}
	v.Set("v", strconv.Itoa(Version))
	v.Set("type", "search")
	v.Set("arg", arg)
	if by != "" && by != DefaultBy {
		v.Set("by", string(by))
	}
	return u.do(ctx, v)
}

func (u *AURUpstream) Suggest(ctx context.Context, arg string, pkgbase bool) ([]string, error) {
	v := url.Values{}
	v.Set("v", strconv.Itoa(Version))
	if pkgbase {
		v.Set("type", "suggest-pkgbase")
	} else {
		v.Set("type", "suggest")
	}
	v.Set("arg", arg)

	body, err := u.get(ctx, v)
	if err != nil {
		return nil, err
	}
	var names []string
	if err := json.Unmarshal(body, &names); err != nil {
		return nil, fmt.Errorf("aurweb: decode upstream suggest: %w", err)
	}
	return names, nil
}

func (u *AURUpstream) do(ctx context.Context, v url.Values) ([]Pkg, error) {
	body, err := u.get(ctx, v)
	if err != nil {
		return nil, err
	}

	var env struct {
		Error   string      `json:"error"`
		Results []rpcResult `json:"results"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("aurweb: decode upstream response: %w", err)
	}
	if env.Error != "" {
		return nil, fmt.Errorf("aurweb: upstream error: %s", env.Error)
	}

	out := make([]Pkg, len(env.Results))
	for i, r := range env.Results {
		out[i] = r.toPkg()
	}
	return out, nil
}

func (u *AURUpstream) get(ctx context.Context, v url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.rpcURL+"?"+v.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", u.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("aurweb: upstream request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("aurweb: upstream status %d", resp.StatusCode)
	}
	return readAllLimited(resp.Body)
}

// rpcResult mirrors the aurweb v5 result schema with nullable fields modelled as
// pointers so JSON null decodes cleanly.
type rpcResult struct {
	ID             int      `json:"ID"`
	Name           string   `json:"Name"`
	PackageBaseID  int      `json:"PackageBaseID"`
	PackageBase    string   `json:"PackageBase"`
	Version        string   `json:"Version"`
	Description    *string  `json:"Description"`
	URL            *string  `json:"URL"`
	NumVotes       int      `json:"NumVotes"`
	Popularity     float64  `json:"Popularity"`
	OutOfDate      *int     `json:"OutOfDate"`
	Maintainer     *string  `json:"Maintainer"`
	Submitter      *string  `json:"Submitter"`
	FirstSubmitted int64    `json:"FirstSubmitted"`
	LastModified   int64    `json:"LastModified"`
	URLPath        string   `json:"URLPath"`
	Depends        []string `json:"Depends"`
	MakeDepends    []string `json:"MakeDepends"`
	CheckDepends   []string `json:"CheckDepends"`
	OptDepends     []string `json:"OptDepends"`
	Conflicts      []string `json:"Conflicts"`
	Provides       []string `json:"Provides"`
	Replaces       []string `json:"Replaces"`
	Groups         []string `json:"Groups"`
	License        []string `json:"License"`
	Keywords       []string `json:"Keywords"`
	CoMaintainers  []string `json:"CoMaintainers"`
}

func (r rpcResult) toPkg() Pkg {
	p := Pkg{
		ID:             r.ID,
		Name:           r.Name,
		PackageBaseID:  r.PackageBaseID,
		PackageBase:    r.PackageBase,
		Version:        r.Version,
		Description:    derefStr(r.Description),
		URL:            derefStr(r.URL),
		NumVotes:       r.NumVotes,
		Popularity:     r.Popularity,
		Maintainer:     derefStr(r.Maintainer),
		Submitter:      derefStr(r.Submitter),
		FirstSubmitted: r.FirstSubmitted,
		LastModified:   r.LastModified,
		URLPath:        r.URLPath,
		Depends:        r.Depends,
		MakeDepends:    r.MakeDepends,
		CheckDepends:   r.CheckDepends,
		OptDepends:     r.OptDepends,
		Conflicts:      r.Conflicts,
		Provides:       r.Provides,
		Replaces:       r.Replaces,
		Groups:         r.Groups,
		License:        r.License,
		Keywords:       r.Keywords,
		CoMaintainers:  r.CoMaintainers,
	}
	if r.OutOfDate != nil {
		p.OutOfDate = *r.OutOfDate
	}
	return p
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func deriveOrigin(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return DefaultAURBase
	}
	return parsed.Scheme + "://" + parsed.Host
}
