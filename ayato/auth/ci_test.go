package auth

import (
	"context"
	"net/http"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/conf"
)

func apiKeyAuthorizer(keys ...conf.CIAPIKey) *CIAuthorizer {
	return &CIAuthorizer{apikey: newAPIKeyAuth(keys)}
}

func TestAPIKeyAuthorize(t *testing.T) {
	a := newAPIKeyAuth([]conf.CIAPIKey{
		{Name: "alter", Key: "secret-1", PublishRepos: []string{"alterlinux"}},
		{Name: "any", Key: "secret-2", PublishRepos: []string{"*"}},
	})

	cases := []struct {
		name, key, repo string
		want            bool
	}{
		{"good key, allowed repo", "secret-1", "alterlinux", true},
		{"good key, other repo", "secret-1", "extra", false},
		{"wildcard key, any repo", "secret-2", "whatever", true},
		{"wrong key", "nope", "alterlinux", false},
		{"empty key", "", "alterlinux", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := a.authorize(tc.key, tc.repo); ok != tc.want {
				t.Fatalf("authorize(%q,%q)=%v, want %v", tc.key, tc.repo, ok, tc.want)
			}
		})
	}
}

func TestAuthorizeRouting(t *testing.T) {
	a := apiKeyAuthorizer(conf.CIAPIKey{Name: "k", Key: "secret", PublishRepos: []string{"alterlinux"}})

	t.Run("X-API-Key routes to api key", func(t *testing.T) {
		h := http.Header{"X-Api-Key": {"secret"}}
		if out, _ := a.Authorize(context.Background(), h, "alterlinux"); out != CIOutcomeAllow {
			t.Fatalf("outcome=%v, want allow", out)
		}
	})
	t.Run("bad X-API-Key denies, no fallthrough", func(t *testing.T) {
		h := http.Header{"X-Api-Key": {"wrong"}}
		if out, _ := a.Authorize(context.Background(), h, "alterlinux"); out != CIOutcomeDeny {
			t.Fatalf("outcome=%v, want deny", out)
		}
	})
	t.Run("Bearer JWT with no OIDC configured denies", func(t *testing.T) {
		h := http.Header{"Authorization": {"Bearer aaa.bbb.ccc"}}
		if out, _ := a.Authorize(context.Background(), h, "alterlinux"); out != CIOutcomeDeny {
			t.Fatalf("outcome=%v, want deny", out)
		}
	})
	t.Run("two-part Bearer is not routed to OIDC", func(t *testing.T) {
		// ayato's own HMAC token is two segments; it must not reach the OIDC path.
		h := http.Header{"Authorization": {"Bearer payload.sig"}}
		if out, _ := a.Authorize(context.Background(), h, "alterlinux"); out != CIOutcomeNone {
			t.Fatalf("outcome=%v, want none", out)
		}
	})
	t.Run("no credential yields none", func(t *testing.T) {
		if out, _ := a.Authorize(context.Background(), http.Header{}, "alterlinux"); out != CIOutcomeNone {
			t.Fatalf("outcome=%v, want none", out)
		}
	})
}

func TestAuthorizeServiceScope(t *testing.T) {
	a := apiKeyAuthorizer(
		conf.CIAPIKey{Name: "publisher", Key: "publish-only", PublishRepos: []string{"core"}},
		conf.CIAPIKey{Name: "signer", Key: "signer-key", Scopes: []string{"signer:register"}},
		conf.CIAPIKey{Name: "root-service", Key: "wildcard", Scopes: []string{"*"}},
	)

	cases := []struct {
		name, key, scope string
		outcome          CIOutcome
		principal        string
	}{
		{name: "named signer scope", key: "signer-key", scope: "signer:register", outcome: CIOutcomeAllow, principal: "signer"},
		{name: "publish grant is not service scope", key: "publish-only", scope: "signer:register", outcome: CIOutcomeDeny},
		{name: "wildcard scope", key: "wildcard", scope: "signer:register", outcome: CIOutcomeAllow, principal: "root-service"},
		{name: "invalid presented key is terminal", key: "wrong", scope: "signer:register", outcome: CIOutcomeDeny},
		{name: "no key permits admin fallback", scope: "signer:register", outcome: CIOutcomeNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := make(http.Header)
			if tc.key != "" {
				h.Set("X-API-Key", tc.key)
			}
			outcome, principal := a.AuthorizeScope(h, tc.scope)
			if outcome != tc.outcome {
				t.Fatalf("outcome = %v, want %v", outcome, tc.outcome)
			}
			if tc.principal != "" && (principal == nil || principal.ID != tc.principal) {
				t.Fatalf("principal = %#v, want %q", principal, tc.principal)
			}
		})
	}
}

func oidcWith(pubs ...conf.CIOIDCPublisher) *oidcAuth {
	a := &oidcAuth{}
	for _, p := range pubs {
		e := oidcPublisher{repository: p.Repository, repositoryID: p.RepositoryID, refs: map[string]bool{}, repos: map[string]bool{}}
		for _, r := range p.AllowRefs {
			e.refs[r] = true
		}
		for _, r := range p.PublishRepos {
			e.repos[r] = true
		}
		a.publishers = append(a.publishers, e)
	}
	return a
}

func TestOIDCAuthorizeClaims(t *testing.T) {
	pub := conf.CIOIDCPublisher{
		Repository:   "FascodeNet/alterlinux-repo",
		AllowRefs:    []string{"refs/heads/main"},
		PublishRepos: []string{"alterlinux"},
	}
	a := oidcWith(pub)

	base := oidcClaims{
		Repository: "FascodeNet/alterlinux-repo",
		Sub:        "repo:FascodeNet/alterlinux-repo:ref:refs/heads/main",
		Ref:        "refs/heads/main",
		EventName:  "push",
	}

	t.Run("matching push allowed", func(t *testing.T) {
		if _, ok := a.authorizeClaims(base, "alterlinux"); !ok {
			t.Fatal("want allow")
		}
	})
	t.Run("pull_request event rejected", func(t *testing.T) {
		c := base
		c.EventName = "pull_request"
		if _, ok := a.authorizeClaims(c, "alterlinux"); ok {
			t.Fatal("pull_request must be rejected")
		}
	})
	t.Run("pull_request sub rejected", func(t *testing.T) {
		c := base
		c.Sub = "repo:FascodeNet/alterlinux-repo:pull_request"
		if _, ok := a.authorizeClaims(c, "alterlinux"); ok {
			t.Fatal("pull_request sub must be rejected")
		}
	})
	t.Run("disallowed ref rejected", func(t *testing.T) {
		c := base
		c.Ref = "refs/heads/dev"
		c.Sub = "repo:FascodeNet/alterlinux-repo:ref:refs/heads/dev"
		if _, ok := a.authorizeClaims(c, "alterlinux"); ok {
			t.Fatal("non-allowlisted ref must be rejected")
		}
	})
	t.Run("other repository rejected", func(t *testing.T) {
		c := base
		c.Repository = "FascodeNet/evil"
		if _, ok := a.authorizeClaims(c, "alterlinux"); ok {
			t.Fatal("other repository must be rejected")
		}
	})
	t.Run("disallowed ayato repo rejected", func(t *testing.T) {
		if _, ok := a.authorizeClaims(base, "extra"); ok {
			t.Fatal("ayato repo outside publish_repos must be rejected")
		}
	})
}

func TestOIDCWildcardPublishReposAllowsAny(t *testing.T) {
	a := oidcWith(conf.CIOIDCPublisher{
		Repository:   "FascodeNet/alterlinux-repo",
		AllowRefs:    []string{"refs/heads/main"},
		PublishRepos: []string{"*"},
	})
	c := oidcClaims{
		Repository: "FascodeNet/alterlinux-repo",
		Sub:        "repo:FascodeNet/alterlinux-repo:ref:refs/heads/main",
		Ref:        "refs/heads/main",
		EventName:  "push",
	}
	for _, repo := range []string{"alterlinux", "extra", "anything"} {
		if _, ok := a.authorizeClaims(c, repo); !ok {
			t.Fatalf("publish_repos=[\"*\"] must allow any pacman repo, got deny for %q", repo)
		}
	}
	// ref gating still applies even with the wildcard.
	bad := c
	bad.Ref = "refs/heads/dev"
	if _, ok := a.authorizeClaims(bad, "alterlinux"); ok {
		t.Fatal("ref gating must still apply with publish_repos=[\"*\"]")
	}
}

func TestOIDCRepositoryIDExactMatch(t *testing.T) {
	a := oidcWith(conf.CIOIDCPublisher{
		RepositoryID: "12345",
		AllowRefs:    []string{"refs/heads/main"},
		PublishRepos: []string{"alterlinux"},
	})
	c := oidcClaims{
		Repository:   "FascodeNet/alterlinux-repo-renamed",
		RepositoryID: "12345",
		Sub:          "repo:FascodeNet/alterlinux-repo-renamed:ref:refs/heads/main",
		Ref:          "refs/heads/main",
		EventName:    "push",
	}
	if _, ok := a.authorizeClaims(c, "alterlinux"); !ok {
		t.Fatal("repository_id match must allow even after rename")
	}
	c.RepositoryID = "99999"
	if _, ok := a.authorizeClaims(c, "alterlinux"); ok {
		t.Fatal("wrong repository_id must be rejected")
	}
}
