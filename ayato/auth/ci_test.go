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
	for _, test := range []struct {
		name    string
		headers http.Header
		want    CIOutcome
	}{
		{
			name:    "X-API-Key routes to api key",
			headers: http.Header{"X-Api-Key": {"secret"}},
			want:    CIOutcomeAllow,
		},
		{
			name:    "bad X-API-Key denies without fallthrough",
			headers: http.Header{"X-Api-Key": {"wrong"}},
			want:    CIOutcomeDeny,
		},
		{
			name:    "Bearer JWT without OIDC denies",
			headers: http.Header{"Authorization": {"Bearer aaa.bbb.ccc"}},
			want:    CIOutcomeDeny,
		},
		{
			name:    "two-part HMAC bearer does not route to OIDC",
			headers: http.Header{"Authorization": {"Bearer payload.sig"}},
			want:    CIOutcomeNone,
		},
		{name: "no credential yields none", headers: http.Header{}, want: CIOutcomeNone},
	} {
		t.Run(test.name, func(t *testing.T) {
			outcome, _ := a.Authorize(
				context.Background(),
				test.headers,
				"alterlinux",
			)
			if outcome != test.want {
				t.Fatalf("outcome = %v, want %v", outcome, test.want)
			}
		})
	}
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
