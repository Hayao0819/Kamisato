package middleware

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/ciauth"
	"github.com/Hayao0819/Kamisato/ayato/ratelimit"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

// adminChecker keeps the middleware from touching kv or the allowlist directly.
type adminChecker interface {
	IsAdmin(id int64) bool
}

type denylistChecker interface {
	IsRevoked(jti string) bool
}

// logTokenConsumer redeems a one-time SSE log token, returning its bound job id;
// ok is false if the token is absent, unknown, or already spent.
type logTokenConsumer interface {
	ConsumeLogToken(token string) (jobID string, ok bool)
}

type Middleware struct {
	cfg       *conf.AyatoConfig
	checker   adminChecker // nil = auth unconfigured; RequireAdmin fails closed (503)
	signer    *auth.Signer
	ci        *ciauth.Authorizer // nil = no CI auth
	denylist  denylistChecker    // nil = per-token revocation not wired
	logTokens logTokenConsumer   // nil = one-time SSE log tokens not wired
	// limiter is the shared kv-backed rate limiter; nil leaves RateLimit a
	// pass-through. rlScope gives each RateLimit call site a distinct scope so
	// independent route limiters do not share one counter.
	limiter *ratelimit.Limiter
	rlScope atomic.Int64
}

func New(cfg *conf.AyatoConfig) *Middleware {
	return &Middleware{
		cfg: cfg,
	}
}

// WithRateLimiter wires the shared kv-backed limiter so RateLimit enforces
// across replicas; unwired leaves RateLimit a pass-through.
func (m *Middleware) WithRateLimiter(store kv.Store) *Middleware {
	m.limiter = ratelimit.New(store)
	return m
}

func (m *Middleware) WithAuth(checker adminChecker, signer *auth.Signer) *Middleware {
	m.checker = checker
	m.signer = signer
	return m
}

// WithDenylist attaches the per-token revocation check; unwired disables it.
func (m *Middleware) WithDenylist(dl denylistChecker) *Middleware {
	m.denylist = dl
	return m
}

// WithLogTokens attaches the one-time SSE log-token consumer used by
// RequireLogAccess; unwired leaves only bearer/session access to /logs.
func (m *Middleware) WithLogTokens(c logTokenConsumer) *Middleware {
	m.logTokens = c
	return m
}

// revoked reports whether a verified token is individually revoked; a JTI-less
// token (sessions, pre-feature tokens) is never denylisted.
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
			// Hint the client that a refresh (not a full re-login) is enough when the
			// only problem is an expired-but-valid access token.
			if m.expiredAccessToken(c, allowBasic) {
				c.Header(accessTokenExpiredHeader, "1")
			}
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

// logTokenHeader carries the one-time SSE log token when a caller cannot put it in
// the query string; the query param is the primary path for an EventSource URL.
const logTokenHeader = "X-Log-Token" //nolint:gosec // G101: an HTTP header name, not a credential

// accessTokenExpiredHeader flags a 401 caused solely by an expired (but validly
// signed, non-revoked) access token, so the CLI knows to refresh and retry rather
// than prompt for a full re-login.
const accessTokenExpiredHeader = "X-Access-Token-Expired" //nolint:gosec // G101: an HTTP header name, not a credential

// expiredAccessToken reports whether the request carries a Bearer (or Basic
// password, when allowed) access token that is validly signed and non-revoked but
// has expired — the one 401 case a refresh can recover from.
func (m *Middleware) expiredAccessToken(c *gin.Context, allowBasic bool) bool {
	if m.signer == nil {
		return false
	}
	authz := c.GetHeader("Authorization")
	tok := ""
	switch {
	case strings.HasPrefix(authz, "Bearer "):
		tok = strings.TrimPrefix(authz, "Bearer ")
	case allowBasic && strings.HasPrefix(authz, "Basic "):
		if _, pass, err := decodeBasic(authz); err == nil {
			tok = pass
		}
	}
	if tok == "" {
		return false
	}
	for _, typ := range []string{auth.TypCLI, auth.TypBearer} {
		if claims, expired, err := m.signer.VerifyTypAllowExpired(tok, typ); err == nil {
			return expired && !m.revoked(claims)
		}
	}
	return false
}

// RequireLogAccess gates the SSE build-log stream. A browser EventSource cannot
// send a bearer, so it presents a one-time token (query "token" or X-Log-Token)
// bound to this job id and spent on first use; a CLI or same-origin session admin
// may authenticate normally. A presented-but-invalid token fails closed rather
// than falling through to the admin path.
func (m *Middleware) RequireLogAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		if tok := logTokenFromRequest(c); tok != "" {
			if m.logTokens != nil {
				if jobID, ok := m.logTokens.ConsumeLogToken(tok); ok && jobID == c.Param("id") {
					c.Next()
					return
				}
			}
			c.Header("WWW-Authenticate", `Bearer realm="ayato"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// No one-time token: fall back to admin session/bearer, same as RequireAdmin.
		if m.checker == nil || m.signer == nil {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		id, login, via, ok := m.resolve(c, false)
		if !ok {
			c.Header("WWW-Authenticate", `Bearer realm="ayato"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if via == ctxViaSession && !m.sameOriginRequest(c) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
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

// logTokenFromRequest reads the one-time log token from the query string (the path
// an EventSource URL uses) or the X-Log-Token header.
func logTokenFromRequest(c *gin.Context) string {
	if tok := c.Query("token"); tok != "" {
		return tok
	}
	return c.GetHeader(logTokenHeader)
}

// sameOriginRequest is the CSRF gate for the cookie path: Sec-Fetch-Site is
// authoritative when present, else an Origin/Referer allowlist; fails closed when
// no origin can be resolved.
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

// allowedOrigin matches an origin case-insensitively against the
// PublicOrigin/SelfOrigin allowlist, normalizing each so a trailing slash does
// not cause a spurious mismatch.
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
