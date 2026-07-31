package aurweb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// defaultUserAgent is sent on every upstream request; the AUR blocks the default
// user agents of common HTTP libraries.
const defaultUserAgent = "kamisato-aurweb/1.0 (+https://github.com/Hayao0819/Kamisato)"

const defaultAURBase = "https://aur.archlinux.org"

// upstreamBatchSize bounds pkgnames per info GET to match AUR helper convention and URL length limits.
const upstreamBatchSize = 150

const upstreamGETAttempts = 4

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

// WithHTTPClient replaces both clients used for RPC and dump requests.
func WithHTTPClient(client *http.Client) AURUpstreamOption {
	return func(u *AURUpstream) {
		if client != nil {
			u.client = client
			u.dumpClient = client
		}
	}
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
		rpcURL = defaultAURBase + "/rpc"
	}
	rpcURL = strings.TrimRight(rpcURL, "/")
	rpcURL = strings.TrimSuffix(rpcURL, "?")

	u := &AURUpstream{
		rpcURL:     rpcURL,
		gitBase:    deriveOrigin(rpcURL),
		userAgent:  defaultUserAgent,
		client:     &http.Client{Timeout: 15 * time.Second},
		dumpClient: &http.Client{Timeout: 3 * time.Minute},
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

	resp, err := doUpstreamGET(u.client, req) //nolint:gosec // upstream RPC host is operator-configured; only query params vary
	if err != nil {
		return nil, fmt.Errorf("aurweb: upstream request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("aurweb: upstream status %d", resp.StatusCode)
	}
	return readAllLimited(resp.Body)
}

func doUpstreamGET(client *http.Client, req *http.Request) (*http.Response, error) {
	var lastErr error
	for attempt := range upstreamGETAttempts {
		resp, err := client.Do(req.Clone(req.Context())) //nolint:gosec // The operator selects the upstream; retries only clone that request.
		if err == nil && (!retryableUpstreamStatus(resp.StatusCode) || attempt+1 == upstreamGETAttempts) {
			return resp, nil
		}
		if resp != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 32<<10))
			_ = resp.Body.Close()
		}
		if err != nil {
			lastErr = err
			if attempt+1 == upstreamGETAttempts {
				return nil, err
			}
		}
		if ctxErr := req.Context().Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if err := waitUpstreamRetry(req.Context(), retryAfter(resp, attempt)); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func retryableUpstreamStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

func retryAfter(resp *http.Response, attempt int) time.Duration {
	if resp != nil {
		value := resp.Header.Get("Retry-After")
		if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 {
			if seconds >= 30 {
				return 30 * time.Second
			}
			return time.Duration(seconds) * time.Second
		}
		if date, err := http.ParseTime(value); err == nil {
			if delay := time.Until(date); delay > 0 {
				return min(delay, 30*time.Second)
			}
		}
	}
	return 100 * time.Millisecond * time.Duration(1<<attempt)
}

func waitUpstreamRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func deriveOrigin(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return defaultAURBase
	}
	return parsed.Scheme + "://" + parsed.Host
}
