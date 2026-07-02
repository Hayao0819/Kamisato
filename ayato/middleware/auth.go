package middleware

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/ciauth"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/gin-gonic/gin"
)

// adminChecker is the narrow contract the middleware depends on, so it never
// touches kv or the allowlist directly.
type adminChecker interface {
	IsAdmin(id int64) bool
}

// denylistChecker reports whether a token id has been revoked.
type denylistChecker interface {
	IsRevoked(jti string) bool
}

type Middleware struct {
	cfg      *conf.AyatoConfig
	checker  adminChecker // nil = auth unconfigured; RequireAdmin fails closed (503)
	signer   *auth.Signer
	ci       *ciauth.Authorizer // nil = no CI auth
	denylist denylistChecker    // nil = per-token revocation not wired
}

func New(cfg *conf.AyatoConfig) *Middleware {
	return &Middleware{
		cfg: cfg,
	}
}

func (m *Middleware) WithAuth(checker adminChecker, signer *auth.Signer) *Middleware {
	m.checker = checker
	m.signer = signer
	return m
}

// WithDenylist attaches the per-token revocation check. Unwired (nil) means no
// per-token revocation, same as before this feature.
func (m *Middleware) WithDenylist(dl denylistChecker) *Middleware {
	m.denylist = dl
	return m
}

// revoked reports whether a verified token has been individually revoked. Tokens
// without a JTI (sessions, pre-feature tokens) are never denylisted here.
func (m *Middleware) revoked(c *auth.Claims) bool {
	return m.denylist != nil && c.JTI != "" && m.denylist.IsRevoked(c.JTI)
}

// Gin context keys for the resolved requester (audit logging).
const (
	ctxGitHubID = "auth_github_id"
	ctxLogin    = "auth_login"
	ctxVia      = "auth_via" // "session" | "bearer" | "basic"
)

// RequireAdmin authenticates the requester and re-checks the admin allowlist,
// fail-closed. allowBasic also accepts a CLI token in HTTP Basic's password
// field, for blinky-compatible routes.
func (m *Middleware) RequireAdmin(allowBasic bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.checker == nil || m.signer == nil {
			// Auth not configured (no signer): fail closed rather than open.
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}

		id, login, via, ok := m.resolve(c, allowBasic)
		if !ok {
			c.Header("WWW-Authenticate", `Bearer realm="ayato"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// CSRF defense-in-depth for the cookie path: cross-site requests aren't
		// same-origin. Bearer/basic (non-browser) callers skip this.
		if via == ctxViaSession && !m.sameOriginRequest(c) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		// Re-check the allowlist every request so a de-allowlisted admin is locked
		// out immediately; this is what makes stateless tokens revocable.
		if !m.checker.IsAdmin(id) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Set(ctxGitHubID, id)
		c.Set(ctxLogin, login)
		c.Set(ctxVia, via)
		c.Next()
	}
}

const (
	ctxViaSession = "session"
	ctxViaBearer  = "bearer"
	ctxViaBasic   = "basic"
)

// sameOriginRequest is the CSRF gate for the cookie path. Sec-Fetch-Site, when
// the browser sends it, is authoritative. When it is absent, fall back to an
// Origin/Referer allowlist so a legitimate same-origin request is not blocked
// while a cross-site one still is. Fails closed when no origin can be resolved.
func (m *Middleware) sameOriginRequest(c *gin.Context) bool {
	if sfs := c.GetHeader("Sec-Fetch-Site"); sfs != "" {
		return sfs == "same-origin"
	}
	origin := c.GetHeader("Origin")
	if origin == "" {
		origin = originOfURL(c.GetHeader("Referer"))
	}
	if origin == "" {
		return false
	}
	return m.allowedOrigin(origin)
}

// allowedOrigin matches an origin (scheme://host) case-insensitively against the
// configured PublicOrigin/SelfOrigin allowlist, normalizing each so a trailing
// slash does not cause a spurious mismatch.
func (m *Middleware) allowedOrigin(origin string) bool {
	if m.cfg == nil {
		return false
	}
	for _, allowed := range []string{m.cfg.Auth.PublicOrigin, m.cfg.Auth.SelfOrigin} {
		if allowed != "" && strings.EqualFold(origin, originOfURL(allowed)) {
			return true
		}
	}
	return false
}

// originOfURL returns the scheme://host of a URL, or "" if it cannot be parsed.
func originOfURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func (m *Middleware) resolve(c *gin.Context, allowBasic bool) (id int64, login, via string, ok bool) {
	if sid, err := c.Cookie(m.cfg.Auth.CookieName()); err == nil && sid != "" {
		if claims, verr := m.signer.VerifyTyp(sid, auth.TypSession); verr == nil {
			return claims.GitHubID, claims.Login, ctxViaSession, true
		}
	}

	authz := c.GetHeader("Authorization")

	// Bearer carries both the CLI token and the SPA's bearer token; accept either.
	if strings.HasPrefix(authz, "Bearer ") {
		tok := strings.TrimPrefix(authz, "Bearer ")
		for _, typ := range []string{auth.TypCLI, auth.TypBearer} {
			if claims, terr := m.signer.VerifyTyp(tok, typ); terr == nil {
				if m.revoked(claims) {
					return 0, "", "", false
				}
				return claims.GitHubID, claims.Login, ctxViaBearer, true
			}
		}
	}

	// blinky-compatible HTTP Basic: the password field carries a signed CLI token.
	if allowBasic && strings.HasPrefix(authz, "Basic ") {
		if _, pass, perr := decodeBasic(authz); perr == nil {
			if claims, terr := m.signer.VerifyTyp(pass, auth.TypCLI); terr == nil {
				if m.revoked(claims) {
					return 0, "", "", false
				}
				return claims.GitHubID, claims.Login, ctxViaBasic, true
			}
		}
	}

	return 0, "", "", false
}

func decodeBasic(header string) (user, pass string, err error) {
	raw, derr := base64.StdEncoding.DecodeString(strings.TrimPrefix(header, "Basic "))
	if derr != nil {
		return "", "", derr
	}
	parts := strings.SplitN(string(raw), ":", 2)
	if len(parts) != 2 {
		return "", "", errBadBasic
	}
	return parts[0], parts[1], nil
}

var errBadBasic = errBasic("malformed basic auth")

type errBasic string

func (e errBasic) Error() string { return string(e) }
