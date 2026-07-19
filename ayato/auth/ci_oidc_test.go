package auth

import (
	"testing"

	"github.com/Hayao0819/Kamisato/internal/conf"
)

func oidcWith(publishers ...conf.CIOIDCPublisher) *oidcAuth {
	return &oidcAuth{publishers: compileOIDCPublishers(publishers)}
}

func TestOIDCAuthorizeClaims(t *testing.T) {
	publisher := conf.CIOIDCPublisher{
		Repository:   "FascodeNet/alterlinux-repo",
		AllowRefs:    []string{"refs/heads/main"},
		PublishRepos: []string{"alterlinux"},
	}
	authorizer := oidcWith(publisher)
	base := oidcClaims{
		Repository: "FascodeNet/alterlinux-repo",
		Sub:        "repo:FascodeNet/alterlinux-repo:ref:refs/heads/main",
		Ref:        "refs/heads/main",
		EventName:  "push",
	}

	tests := []struct {
		name   string
		claims oidcClaims
		repo   string
		want   bool
	}{
		{name: "matching push", claims: base, repo: "alterlinux", want: true},
		{
			name: "pull request event",
			claims: func() oidcClaims {
				value := base
				value.EventName = "pull_request"
				return value
			}(),
			repo: "alterlinux",
		},
		{
			name: "pull request subject",
			claims: func() oidcClaims {
				value := base
				value.Sub = "repo:FascodeNet/alterlinux-repo:pull_request"
				return value
			}(),
			repo: "alterlinux",
		},
		{
			name: "disallowed ref",
			claims: func() oidcClaims {
				value := base
				value.Ref = "refs/heads/dev"
				value.Sub = "repo:FascodeNet/alterlinux-repo:ref:refs/heads/dev"
				return value
			}(),
			repo: "alterlinux",
		},
		{
			name: "other repository",
			claims: func() oidcClaims {
				value := base
				value.Repository = "FascodeNet/evil"
				return value
			}(),
			repo: "alterlinux",
		},
		{name: "disallowed ayato repo", claims: base, repo: "extra"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, allowed := authorizer.authorizeClaims(test.claims, test.repo); allowed != test.want {
				t.Fatalf("allowed = %v, want %v", allowed, test.want)
			}
		})
	}
}

func TestOIDCWildcardPublishReposAllowsAny(t *testing.T) {
	authorizer := oidcWith(conf.CIOIDCPublisher{
		Repository:   "FascodeNet/alterlinux-repo",
		AllowRefs:    []string{"refs/heads/main"},
		PublishRepos: []string{"*"},
	})
	claims := oidcClaims{
		Repository: "FascodeNet/alterlinux-repo",
		Sub:        "repo:FascodeNet/alterlinux-repo:ref:refs/heads/main",
		Ref:        "refs/heads/main",
		EventName:  "push",
	}
	for _, repo := range []string{"alterlinux", "extra", "anything"} {
		if _, allowed := authorizer.authorizeClaims(claims, repo); !allowed {
			t.Fatalf("wildcard publish repo denied %q", repo)
		}
	}
	claims.Ref = "refs/heads/dev"
	if _, allowed := authorizer.authorizeClaims(claims, "alterlinux"); allowed {
		t.Fatal("ref gating must still apply with a publish wildcard")
	}
}

func TestOIDCRepositoryIDExactMatch(t *testing.T) {
	authorizer := oidcWith(conf.CIOIDCPublisher{
		RepositoryID: "12345",
		AllowRefs:    []string{"refs/heads/main"},
		PublishRepos: []string{"alterlinux"},
	})
	claims := oidcClaims{
		Repository:   "FascodeNet/alterlinux-repo-renamed",
		RepositoryID: "12345",
		Sub:          "repo:FascodeNet/alterlinux-repo-renamed:ref:refs/heads/main",
		Ref:          "refs/heads/main",
		EventName:    "push",
	}
	if _, allowed := authorizer.authorizeClaims(claims, "alterlinux"); !allowed {
		t.Fatal("repository_id match must allow a renamed repository")
	}
	claims.RepositoryID = "99999"
	if _, allowed := authorizer.authorizeClaims(claims, "alterlinux"); allowed {
		t.Fatal("wrong repository_id must be rejected")
	}
}
