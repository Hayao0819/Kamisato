package middleware

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/ciauth"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/gin-gonic/gin"
)

// adminChecker is the slice of the service the middleware needs: a fail-closed
// per-request allowlist check. The service satisfies it, but the middleware
// depends only on this narrow contract — never on kv or the allowlist directly.
type adminChecker interface {
	IsAdmin(id int64) bool
}

type Middleware struct {
	cfg     *conf.AyatoConfig
	checker adminChecker       // nil = auth unconfigured; RequireAdmin fails closed (503)
	signer  *auth.Signer       // verifies stateless session/CLI tokens
	ci      *ciauth.Authorizer // CI publish credentials; nil = no CI auth
}

func New(cfg *conf.AyatoConfig) *Middleware {
	return &Middleware{
		cfg: cfg,
	}
}

// WithAuth attaches the admin checker (the service) and the stateless signer
// used to verify sessions/CLI tokens and to re-check the allowlist on every
// request.
func (m *Middleware) WithAuth(checker adminChecker, signer *auth.Signer) *Middleware {
	m.checker = checker
	m.signer = signer
	return m
}

// Gin context keys for the resolved requester (audit logging).
const (
	ctxGitHubID = "auth_github_id"
	ctxLogin    = "auth_login"
	ctxVia      = "auth_via" // "session" | "bearer" | "basic"
)

// RequireAdmin authenticates the requester and enforces the admin allowlist
// (fail-closed). Resolution order: (1) session cookie, (2) Authorization:
// Bearer <clitoken>. The cookie path additionally requires
// Sec-Fetch-Site: same-origin as CSRF defense-in-depth; the bearer (non-browser)
// path skips that check. allowBasic enables an extra resolver where HTTP Basic's
// password field carries a CLI token (username ignored) — used only on the
// blinky-compatible routes so existing clients keep working.
func (m *Middleware) RequireAdmin(allowBasic bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.checker == nil || m.signer == nil {
			// Auth is not configured (no session secret / signer). Fail closed:
			// never allow a mutating or admin route through unauthenticated.
			// Configure auth.github + auth.session_secret to enable them; until
			// then they are unavailable rather than open.
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}

		id, login, via, ok := m.resolve(c, allowBasic)
		if !ok {
			c.Header("WWW-Authenticate", `Bearer realm="ayato"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// CSRF defense-in-depth for the cookie path only: browsers send
		// Sec-Fetch-Site, and a cross-site form/navigation will not be
		// same-origin. Non-browser callers (bearer/basic token) skip this.
		if via == ctxViaSession && c.GetHeader("Sec-Fetch-Site") != "same-origin" {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		// Re-check the allowlist on every request (fail-closed): a de-allowlisted
		// admin is locked out immediately, even with a live signed session/token.
		// This is what makes stateless tokens revocable.
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

// resolve maps the request to a GitHub id via, in order: signed session cookie,
// signed Bearer CLI token, then (when allowBasic) HTTP Basic with the signed
// token in the password field. Every credential is a stateless signed envelope;
// resolution returns ok=false when nothing verifies.
func (m *Middleware) resolve(c *gin.Context, allowBasic bool) (id int64, login, via string, ok bool) {
	// (a) signed session cookie
	if sid, err := c.Cookie(m.cfg.Auth.CookieName()); err == nil && sid != "" {
		if claims, verr := m.signer.VerifyTyp(sid, auth.TypSession); verr == nil {
			return claims.GitHubID, claims.Login, ctxViaSession, true
		}
	}

	authz := c.GetHeader("Authorization")

	// (b) Authorization: Bearer <signed token>. Both the CLI token and the web
	// SPA's bearer token ride this header; accept either type.
	if strings.HasPrefix(authz, "Bearer ") {
		tok := strings.TrimPrefix(authz, "Bearer ")
		for _, typ := range []string{auth.TypCLI, auth.TypBearer} {
			if claims, terr := m.signer.VerifyTyp(tok, typ); terr == nil {
				return claims.GitHubID, claims.Login, ctxViaBearer, true
			}
		}
	}

	// (c) blinky-compatible HTTP Basic: password field carries a signed CLI token.
	if allowBasic && strings.HasPrefix(authz, "Basic ") {
		if _, pass, perr := decodeBasic(authz); perr == nil {
			if claims, terr := m.signer.VerifyTyp(pass, auth.TypCLI); terr == nil {
				return claims.GitHubID, claims.Login, ctxViaBasic, true
			}
		}
	}

	return 0, "", "", false
}

// decodeBasic decodes "Basic <base64(user:pass)>" into (user, pass).
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
