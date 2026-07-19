package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// CIPrincipal identifies an authorized CI caller. CI authorization admits a
// non-interactive upload by a repository identity (not a GitHub user) scoped to
// specific repos: credentials route by header shape to exactly one verifier, and a
// presented-but-invalid credential is terminal (no weaker fallback), which keeps the
// routing safe.
type CIPrincipal struct {
	Via string // "apikey" | "oidc"
	ID  string // key name, or the GitHub repository
}

type CIOutcome int

const (
	// CIOutcomeNone means no CI credential was presented; the caller may try admin auth.
	CIOutcomeNone CIOutcome = iota
	// CIOutcomeDeny means a CI credential was presented but is invalid or unauthorized.
	CIOutcomeDeny
	// CIOutcomeAllow means the request is authorized.
	CIOutcomeAllow
)

// CIAuthorizer holds the configured CI verifiers. Construct once and reuse: the
// OIDC backend keeps a long-lived JWKS-caching verifier.
type CIAuthorizer struct {
	apikey *apiKeyAuth
	oidc   *oidcAuth
}

// NewCIAuthorizer performs OIDC issuer discovery when OIDC is enabled, so it makes a
// network call and may fail at startup.
func NewCIAuthorizer(ctx context.Context, cfg conf.CIAuthConfig) (*CIAuthorizer, error) {
	a := &CIAuthorizer{}
	if len(cfg.APIKeys) > 0 {
		a.apikey = newAPIKeyAuth(cfg.APIKeys)
	}
	if cfg.GitHubOIDC.Enabled {
		o, err := newOIDCAuth(ctx, cfg.GitHubOIDC)
		if err != nil {
			return nil, errors.WrapErr(err, "ci oidc init")
		}
		a.oidc = o
	}
	return a, nil
}

func (a *CIAuthorizer) Enabled() bool {
	return a != nil && (a.apikey != nil || a.oidc != nil)
}

// Authorize routes the request to exactly one CI verifier by header shape and
// authorizes publishing to ayatoRepo.
//
//   - X-API-Key present            -> API key verifier only
//   - Authorization: Bearer <jwt>  -> OIDC verifier only (3 segments / 2 dots)
//   - neither                      -> CIOutcomeNone (let the caller try admin auth)
//
// A failed or unconfigured verifier yields CIOutcomeDeny; there is no fallthrough.
func (a *CIAuthorizer) Authorize(ctx context.Context, h http.Header, ayatoRepo string) (CIOutcome, *CIPrincipal) {
	if k := h.Get("X-API-Key"); k != "" {
		if a.apikey == nil {
			return CIOutcomeDeny, nil
		}
		if p, ok := a.apikey.authorize(k, ayatoRepo); ok {
			return CIOutcomeAllow, p
		} else {
			return CIOutcomeDeny, p
		}
	}

	if tok, ok := bearerJWT(h); ok {
		if a.oidc == nil {
			return CIOutcomeDeny, nil
		}
		if p, ok := a.oidc.authorize(ctx, tok, ayatoRepo); ok {
			return CIOutcomeAllow, p
		}
		return CIOutcomeDeny, nil
	}

	return CIOutcomeNone, nil
}

// AuthorizeScope authenticates an API key for a service scope.
func (a *CIAuthorizer) AuthorizeScope(h http.Header, scope string) (CIOutcome, *CIPrincipal) {
	key := h.Get("X-API-Key")
	if key == "" {
		return CIOutcomeNone, nil
	}
	if a == nil || a.apikey == nil {
		return CIOutcomeDeny, nil
	}
	principal, ok := a.apikey.authorizeScope(key, scope)
	if ok {
		return CIOutcomeAllow, principal
	}
	return CIOutcomeDeny, principal
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
