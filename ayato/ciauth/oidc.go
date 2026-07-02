package ciauth

import (
	"context"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/internal/httpx"
	"github.com/coreos/go-oidc/v3/oidc"
)

const githubOIDCIssuer = "https://token.actions.githubusercontent.com"

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

func newOIDCAuth(ctx context.Context, cfg conf.CIGitHubOIDC) (*oidcAuth, error) {
	client := httpx.New(10*time.Second, 3)
	provider, err := oidc.NewProvider(oidc.ClientContext(ctx, client), githubOIDCIssuer)
	if err != nil {
		return nil, errwrap.WrapErr(err, "discover github oidc issuer")
	}
	// Pin RS256 (GitHub signs only RS256) and require the configured audience;
	// never skip the issuer/audience/signature checks.
	verifier := provider.Verifier(&oidc.Config{
		ClientID:             cfg.Audience,
		SupportedSigningAlgs: []string{oidc.RS256},
	})

	a := &oidcAuth{verifier: verifier}
	for _, p := range cfg.Publishers {
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
		a.publishers = append(a.publishers, e)
	}
	return a, nil
}

// claims are GitHub's OIDC claims. repository_id is a JSON string, not a number.
type claims struct {
	Repository   string `json:"repository"`
	RepositoryID string `json:"repository_id"`
	Sub          string `json:"sub"`
	Ref          string `json:"ref"`
	EventName    string `json:"event_name"`
}

func (a *oidcAuth) authorize(ctx context.Context, raw, repo string) (*Principal, bool) {
	tok, err := a.verifier.Verify(ctx, raw)
	if err != nil {
		return nil, false
	}
	var c claims
	if err := tok.Claims(&c); err != nil {
		return nil, false
	}
	return a.authorizeClaims(c, repo)
}

// authorizeClaims is the authorization decision over already-verified claims.
// It is the security boundary after signature/iss/aud/exp verification.
func (a *oidcAuth) authorizeClaims(c claims, repo string) (*Principal, bool) {
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
		return &Principal{Via: "oidc", ID: c.Repository}, true
	}
	return nil, false
}

// matches requires an exact match on repository_id when set (immutable, survives
// a repo rename), else on the repository slug. No prefix or wildcard matching.
func (p *oidcPublisher) matches(c claims) bool {
	if p.repositoryID != "" {
		return c.RepositoryID == p.repositoryID
	}
	return c.Repository == p.repository
}
