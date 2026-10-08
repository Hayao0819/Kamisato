package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func oidcWith(publishers ...CIOIDCPublisher) *oidcAuth {
	return &oidcAuth{publishers: compileOIDCPublishers(publishers)}
}

func TestOIDCAuthorizeClaims(t *testing.T) {
	publisher := CIOIDCPublisher{
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
	authorizer := oidcWith(CIOIDCPublisher{
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
	authorizer := oidcWith(CIOIDCPublisher{
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

func TestOIDCGETRetryTransportRetriesTemporaryFailure(t *testing.T) {
	attempts := 0
	transport := oidcGETRetryTransport{base: oidcRoundTripFunc(func(*http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Header:     http.Header{"Retry-After": []string{"0"}},
				Body:       io.NopCloser(strings.NewReader("temporary")),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	})}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://issuer.example/.well-known/openid-configuration", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if attempts != 2 || resp.StatusCode != http.StatusOK {
		t.Fatalf("attempts = %d, status = %d", attempts, resp.StatusCode)
	}
}

func TestOIDCGETRetryTransportHonorsCancellation(t *testing.T) {
	transport := oidcGETRetryTransport{base: oidcRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("temporary")
	})}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://issuer.example/.well-known/openid-configuration", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := transport.RoundTrip(req); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestOIDCRetryAfterCapsBeforeDurationConversion(t *testing.T) {
	resp := &http.Response{Header: http.Header{"Retry-After": []string{"9223372036854775807"}}}
	if got := oidcRetryAfter(resp, 0); got != 30*time.Second {
		t.Fatalf("retry delay = %s, want 30s", got)
	}
}

type oidcRoundTripFunc func(*http.Request) (*http.Response, error)

func (f oidcRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestOIDCDiscoveryUsesInjectedClient(t *testing.T) {
	requests := 0
	wantErr := errors.New("injected discovery failure")
	client := &http.Client{Transport: oidcRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.URL.String() != githubOIDCIssuer+"/.well-known/openid-configuration" {
			t.Fatalf("discovery URL = %s", request.URL)
		}
		return nil, wantErr
	})}
	_, err := NewCIAuthorizerWithHTTPClient(context.Background(), CISettings{GitHubOIDC: CIGitHubOIDC{Enabled: true}}, client)
	if !errors.Is(err, wantErr) || requests != oidcGETAttempts {
		t.Fatalf("error = %v; requests = %d", err, requests)
	}
}
