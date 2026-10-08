package auth

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

const githubOIDCIssuer = "https://token.actions.githubusercontent.com"

const oidcGETAttempts = 4

type oidcAuth struct {
	verifier   *oidc.IDTokenVerifier
	publishers []oidcPublisher
}

type oidcPublisher struct {
	repository   string
	repositoryID string
	refs         map[string]bool
	repos        map[string]bool
}

func newOIDCAuth(ctx context.Context, cfg CIGitHubOIDC, client *http.Client) (*oidcAuth, error) {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	// Keep the dedicated GET-only retry policy with either a default or supplied
	// transport, without mutating a client shared with other outbound operations.
	oidcClient := *client
	oidcClient.Transport = oidcGETRetryTransport{base: client.Transport}
	provider, err := oidc.NewProvider(oidc.ClientContext(ctx, &oidcClient), githubOIDCIssuer)
	if err != nil {
		return nil, errors.WrapErr(err, "discover github oidc issuer")
	}
	// Pin RS256 (GitHub signs only RS256) and require the configured audience;
	// never skip the issuer/audience/signature checks.
	verifier := provider.Verifier(&oidc.Config{
		ClientID:             cfg.Audience,
		SupportedSigningAlgs: []string{oidc.RS256},
	})

	return &oidcAuth{
		verifier:   verifier,
		publishers: compileOIDCPublishers(cfg.Publishers),
	}, nil
}

type oidcGETRetryTransport struct {
	base http.RoundTripper
}

func (t oidcGETRetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	if req.Method != http.MethodGet || req.Body != nil {
		return base.RoundTrip(req) //nolint:gosec // OIDC supplies the discovery and JWKS URLs to this dedicated client.
	}

	var lastErr error
	for attempt := range oidcGETAttempts {
		resp, err := base.RoundTrip(req.Clone(req.Context())) //nolint:gosec // Only replayable OIDC GET requests reach this transport.
		if err == nil && (!retryableOIDCStatus(resp.StatusCode) || attempt+1 == oidcGETAttempts) {
			return resp, nil
		}
		if resp != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 32<<10))
			_ = resp.Body.Close()
		}
		if err != nil {
			lastErr = err
			if attempt+1 == oidcGETAttempts {
				return nil, err
			}
		}
		if err := waitOIDCRetry(req.Context(), oidcRetryAfter(resp, attempt)); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func retryableOIDCStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

func oidcRetryAfter(resp *http.Response, attempt int) time.Duration {
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

func waitOIDCRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func compileOIDCPublishers(config []CIOIDCPublisher) []oidcPublisher {
	publishers := make([]oidcPublisher, 0, len(config))
	for _, p := range config {
		e := oidcPublisher{
			repository:   p.Repository,
			repositoryID: p.RepositoryID,
			refs:         map[string]bool{},
			repos:        map[string]bool{},
		}
		for _, r := range p.AllowRefs {
			e.refs[r] = true
		}
		for _, r := range p.PublishRepos {
			e.repos[r] = true
		}
		publishers = append(publishers, e)
	}
	return publishers
}

// oidcClaims are GitHub's OIDC oidcClaims. repository_id is a JSON string, not a number.
type oidcClaims struct {
	Repository   string `json:"repository"`
	RepositoryID string `json:"repository_id"`
	Sub          string `json:"sub"`
	Ref          string `json:"ref"`
	EventName    string `json:"event_name"`
}

func (a *oidcAuth) authorize(ctx context.Context, raw, repo string) (*CIPrincipal, bool) {
	tok, err := a.verifier.Verify(ctx, raw)
	if err != nil {
		return nil, false
	}
	var c oidcClaims
	if err := tok.Claims(&c); err != nil {
		return nil, false
	}
	return a.authorizeClaims(c, repo)
}

// authorizeClaims is the authorization decision over already-verified oidcClaims.
// It is the security boundary after signature/iss/aud/exp verification.
func (a *oidcAuth) authorizeClaims(c oidcClaims, repo string) (*CIPrincipal, bool) {
	// Pull-request runs carry attacker-influenceable refs and must never publish.
	if c.EventName == "pull_request" || c.EventName == "pull_request_target" ||
		strings.HasSuffix(c.Sub, ":pull_request") {
		return nil, false
	}

	for i := range a.publishers {
		p := &a.publishers[i]
		if !p.matches(c) {
			continue
		}
		if !p.refs[c.Ref] {
			continue
		}
		if !p.repos[repo] && !p.repos["*"] {
			continue
		}
		return &CIPrincipal{Via: "oidc", ID: c.Repository}, true
	}
	return nil, false
}

// matches requires an exact match on repository_id when set (immutable, survives
// a repo rename), else on the repository slug. No prefix or wildcard matching.
func (p *oidcPublisher) matches(c oidcClaims) bool {
	if p.repositoryID != "" {
		return c.RepositoryID == p.repositoryID
	}
	return c.Repository == p.repository
}
