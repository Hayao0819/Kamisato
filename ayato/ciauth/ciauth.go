// Package ciauth authorizes non-interactive CI uploads to ayato. A CI caller is
// a repository identity, not a GitHub user, so it is a principal distinct from
// the admin allowlist: it can only publish, scoped to specific ayato repos.
//
// Two credential types are supported and are routed by header shape to exactly
// one verifier. A presented-but-invalid CI credential is terminal (the caller
// must not fall back to a weaker check), which is what makes the routing safe.
package ciauth

import (
	"context"
	"net/http"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
)

// Principal is an authorized CI identity.
type Principal struct {
	Via string // "apikey" | "oidc"
	ID  string // key name, or the GitHub repository
}

// Outcome is the result of routing a request to a CI verifier.
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

// New builds an Authorizer. It performs OIDC issuer discovery when OIDC is
// enabled, so it makes a network call and may fail at startup.
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

// Enabled reports whether any CI method is configured.
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
// A credential routed to a verifier that is unconfigured or rejects it yields
// OutcomeDeny; there is no fallthrough from a failed verifier to a weaker one.
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

// bearerJWT returns the bearer value only when it is shaped like a JWT (three
// dot-separated segments). ayato's own HMAC token is two segments, so this only
// routes; the RS256 signature check in the OIDC verifier is the real boundary.
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
