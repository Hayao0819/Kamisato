// Package ciauth authorizes non-interactive CI uploads. A CI caller is a
// repository identity, not a GitHub user: a distinct principal that may only
// publish to scoped repos. Credentials route by header shape to exactly one
// verifier, and a presented-but-invalid credential is terminal (no fallback to a
// weaker check), which is what makes the routing safe.
package ciauth

import (
	"context"
	"net/http"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
)

type Principal struct {
	Via string // "apikey" | "oidc"
	ID  string // key name, or the GitHub repository
}

type Outcome int

const (
	// OutcomeNone means no CI credential was presented; the caller may try admin auth.
	OutcomeNone Outcome = iota
	// OutcomeDeny means a CI credential was presented but is invalid or unauthorized.
	OutcomeDeny
	// OutcomeAllow means the request is authorized.
	OutcomeAllow
)

// Authorizer holds the configured CI verifiers. Construct once and reuse: the
// OIDC backend keeps a long-lived JWKS-caching verifier.
type Authorizer struct {
	apikey *apiKeyAuth
	oidc   *oidcAuth
}

// New performs OIDC issuer discovery when OIDC is enabled, so it makes a network
// call and may fail at startup.
func New(ctx context.Context, cfg conf.CIAuthConfig) (*Authorizer, error) {
	a := &Authorizer{}
	if len(cfg.APIKeys) > 0 {
		a.apikey = newAPIKeyAuth(cfg.APIKeys)
	}
	if cfg.GitHubOIDC.Enabled {
		o, err := newOIDCAuth(ctx, cfg.GitHubOIDC)
		if err != nil {
			return nil, utils.WrapErr(err, "ci oidc init")
		}
		a.oidc = o
	}
	return a, nil
}

func (a *Authorizer) Enabled() bool {
	return a != nil && (a.apikey != nil || a.oidc != nil)
}

// Authorize routes the request to exactly one CI verifier by header shape and
// authorizes publishing to ayatoRepo.
//
//   - X-API-Key present            -> API key verifier only
//   - Authorization: Bearer <jwt>  -> OIDC verifier only (3 segments / 2 dots)
//   - neither                      -> OutcomeNone (let the caller try admin auth)
//
// A failed or unconfigured verifier yields OutcomeDeny; there is no fallthrough.
func (a *Authorizer) Authorize(ctx context.Context, h http.Header, ayatoRepo string) (Outcome, *Principal) {
	if k := h.Get("X-API-Key"); k != "" {
		if a.apikey == nil {
			return OutcomeDeny, nil
		}
		if p, ok := a.apikey.authorize(k, ayatoRepo); ok {
			return OutcomeAllow, p
		}
		return OutcomeDeny, nil
	}

	if tok, ok := bearerJWT(h); ok {
		if a.oidc == nil {
			return OutcomeDeny, nil
		}
		if p, ok := a.oidc.authorize(ctx, tok, ayatoRepo); ok {
			return OutcomeAllow, p
		}
		return OutcomeDeny, nil
	}

	return OutcomeNone, nil
}

// bearerJWT returns the bearer value only if it's shaped like a JWT (3 dot-separated
// segments). ayato's own HMAC token has 2, so this only routes — the OIDC RS256
// check is the real boundary.
func bearerJWT(h http.Header) (string, bool) {
	authz := h.Get("Authorization")
	if !strings.HasPrefix(authz, "Bearer ") {
		return "", false
	}
	tok := strings.TrimPrefix(authz, "Bearer ")
	if strings.Count(tok, ".") != 2 {
		return "", false
	}
	return tok, true
}
