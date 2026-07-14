package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth/ciauth"
)

const ctxCIPublisher = "auth_ci"

func (m *Middleware) WithCIAuth(ci *ciauth.Authorizer) *Middleware {
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
			case ciauth.OutcomeAllow:
				c.Set(ctxVia, "ci")
				c.Set(ctxCIPublisher, p.ID)
				c.Next()
				return
			case ciauth.OutcomeDeny:
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}
		m.RequireAdmin()(c)
	}
}
