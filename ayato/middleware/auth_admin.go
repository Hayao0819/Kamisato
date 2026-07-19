package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/httpsecurity"
)

// accessTokenExpiredHeader tells a CLI that refreshing can recover the request.
const accessTokenExpiredHeader = "X-Access-Token-Expired" //nolint:gosec // HTTP header name

// RequireAdmin gates a route to an allowlisted admin via session or bearer.
func (m *Middleware) RequireAdmin() gin.HandlerFunc {
	return m.requireAdmin(false)
}

// RequireBlinkyAdmin additionally accepts a CLI token as a Basic password.
func (m *Middleware) RequireBlinkyAdmin() gin.HandlerFunc {
	return m.requireAdmin(true)
}

func (m *Middleware) requireAdmin(allowBasic bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.authorizeAdminRequest(c, allowBasic, true) {
			return
		}
		c.Next()
	}
}

// authorizeAdminRequest is the single admin policy used by ordinary routes and
// the bearer/session fallback for build logs.
func (m *Middleware) authorizeAdminRequest(
	c *gin.Context,
	allowBasic bool,
	hintExpiredToken bool,
) bool {
	if m.checker == nil || m.signer == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return false
	}

	resolver := m.requestResolver()
	identity, ok, err := resolver.Resolve(c.Request, allowBasic)
	if err != nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return false
	}
	if !ok {
		if hintExpiredToken {
			expired, expiryErr := resolver.ExpiredAccessToken(c.Request, allowBasic)
			if expiryErr != nil {
				c.AbortWithStatus(http.StatusServiceUnavailable)
				return false
			}
			if expired {
				c.Header(accessTokenExpiredHeader, "1")
			}
		}
		abortUnauthorized(c)
		return false
	}

	if identity.Via == auth.ViaSession && !m.sameOriginRequest(c) {
		c.AbortWithStatus(http.StatusForbidden)
		return false
	}
	if !m.checker.IsAdmin(identity.GitHubID) {
		c.AbortWithStatus(http.StatusForbidden)
		return false
	}

	c.Set(ctxGitHubID, identity.GitHubID)
	c.Set(ctxLogin, identity.Login)
	c.Set(ctxVia, string(identity.Via))
	return true
}

func abortUnauthorized(c *gin.Context) {
	c.Header("WWW-Authenticate", `Bearer realm="ayato"`)
	c.AbortWithStatus(http.StatusUnauthorized)
}

// sameOriginRequest is the CSRF gate for cookie-authenticated requests.
func (m *Middleware) sameOriginRequest(c *gin.Context) bool {
	if m.cfg == nil {
		return httpsecurity.SameOrigin(c.Request)
	}
	return httpsecurity.SameOrigin(
		c.Request,
		m.cfg.Auth.PublicOrigin,
		m.cfg.Auth.SelfOrigin,
	)
}
