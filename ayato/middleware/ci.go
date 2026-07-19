package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth"
)

const ctxCIPublisher = "auth_ci"

func (m *Middleware) WithCIAuth(ci *auth.CIAuthorizer) *Middleware {
	m.ci = ci
	return m
}

// RequireCI authorizes a CI publisher (API token / OIDC, scoped to :repo) or falls
// back to an admin (session/bearer); a presented-but-failed CI credential is a hard
// 403, never a fallthrough to the admin path.
func (m *Middleware) RequireCI() gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.ci != nil && m.ci.Enabled() {
			outcome, p := m.ci.Authorize(c.Request.Context(), c.Request.Header, c.Param("repo"))
			switch outcome {
			case auth.CIOutcomeAllow:
				slog.Info("Ayato CI publisher authorized", "principal", p.ID, "via", p.Via, "repo", c.Param("repo"), "method", c.Request.Method, "path", c.FullPath())
				c.Set(ctxVia, "ci")
				c.Set(ctxCIPublisher, p.ID)
				c.Next()
				return
			case auth.CIOutcomeDeny:
				if p != nil {
					slog.Warn("Ayato CI publisher repo scope denied", "principal", p.ID, "via", p.Via, "repo", c.Param("repo"), "method", c.Request.Method, "path", c.FullPath())
				}
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}
		m.RequireAdmin()(c)
	}
}

// RequireServiceScope accepts a scoped API key or an admin session.
func (m *Middleware) RequireServiceScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.ci != nil && m.ci.Enabled() {
			outcome, principal := m.ci.AuthorizeScope(c.Request.Header, scope)
			switch outcome {
			case auth.CIOutcomeAllow:
				slog.Info("Ayato service API key authorized", "principal", principal.ID, "scope", scope, "method", c.Request.Method, "path", c.FullPath())
				c.Set(ctxVia, "service")
				c.Set(ctxCIPublisher, principal.ID)
				c.Next()
				return
			case auth.CIOutcomeDeny:
				if principal != nil {
					slog.Warn("Ayato service API key scope denied", "principal", principal.ID, "scope", scope, "method", c.Request.Method, "path", c.FullPath())
				}
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}
		m.RequireAdmin()(c)
	}
}

// RequireSignerRegistration optionally permits the legacy Basic-auth bridge.
func (m *Middleware) RequireSignerRegistration() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("X-API-Key") != "" {
			m.RequireServiceScope("signer:register")(c)
			return
		}
		if m.cfg != nil && m.cfg.Auth.AllowLegacySignerBasic && strings.HasPrefix(c.GetHeader("Authorization"), "Basic ") {
			m.RequireBlinkyAdmin()(c)
			if !c.IsAborted() {
				slog.Warn("Ayato accepted legacy Basic signer registration", "method", c.Request.Method, "path", c.FullPath())
			}
			return
		}
		m.RequireServiceScope("signer:register")(c)
	}
}
