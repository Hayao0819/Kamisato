package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// logTokenHeader carries a one-time SSE token for clients that can set headers.
const logTokenHeader = "X-Log-Token" //nolint:gosec // HTTP header name

// RequireLogAccess accepts a one-time token bound to the requested job. Without
// one, it applies the same session/bearer admin policy as RequireAdmin.
func (m *Middleware) RequireLogAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		if token := logTokenFromRequest(c); token != "" {
			if m.logTokens != nil {
				jobID, ok, err := m.logTokens.ConsumeLogToken(token)
				if err != nil {
					c.AbortWithStatus(http.StatusServiceUnavailable)
					return
				}
				if ok && jobID == c.Param("id") {
					c.Next()
					return
				}
			}
			abortUnauthorized(c)
			return
		}

		if !m.authorizeAdminRequest(c, false, false) {
			return
		}
		c.Next()
	}
}

func logTokenFromRequest(c *gin.Context) string {
	if token := c.Query("token"); token != "" {
		return token
	}
	return c.GetHeader(logTokenHeader)
}
