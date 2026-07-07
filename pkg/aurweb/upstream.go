package aurweb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/httpx"
)

// defaultUserAgent is sent on every upstream request; the AUR blocks the default
// user agents of common HTTP libraries.
const defaultUserAgent = "kamisato-aurweb/1.0 (+https://github.com/Hayao0819/Kamisato)"

const defaultAURBase = "https://aur.archlinux.org"

// upstreamBatchSize bounds pkgnames per info GET to match AUR helper convention and URL length limits.
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
		rpcURL = defaultAURBase + "/rpc"
	}
	rpcURL = strings.TrimRight(rpcURL, "/")
	rpcURL = strings.TrimSuffix(rpcURL, "?")

	u := &AURUpstream{
		rpcURL:     rpcURL,
		gitBase:    deriveOrigin(rpcURL),
		userAgent:  defaultUserAgent,
		client:     httpx.New(15*time.Second, 3),
		dumpClient: httpx.New(3*time.Minute, 3),
	}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

func (u *AURUpstream) GitBase() string { return u.gitBase }

func (u *AURUpstream) Info(ctx context.Context, names []string) ([]Pkg, error) {
	var out []Pkg
	for chunk := range slices.Chunk(names, upstreamBatchSize) {
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.rpcURL+"?"+v.Encode(), nil) //nolint:gosec // upstream RPC host is operator-configured; only query params vary
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", u.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := u.client.Do(req) //nolint:gosec // upstream RPC host is operator-configured; only query params vary
	if err != nil {
		return nil, fmt.Errorf("aurweb: upstream request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("aurweb: upstream status %d", resp.StatusCode)
	}
	return readAllLimited(resp.Body)
}

func deriveOrigin(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return defaultAURBase
	}
	return parsed.Scheme + "://" + parsed.Host
}
