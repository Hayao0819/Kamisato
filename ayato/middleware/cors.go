package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Cors avoids a permissive AllowAllOrigins: the SPA is same-origin behind the
// lumine BFF and the CLI is non-browser, and wildcard origin + credentials is
// forbidden by the spec and a CSRF risk. With PublicOrigin set, allow exactly
// that origin with credentials.
func (m *Middleware) Cors() gin.HandlerFunc {
	if m.cfg == nil || m.cfg.Auth.PublicOrigin == "" {
		// No cross-origin browser access expected; emit no CORS headers.
		return func(c *gin.Context) { c.Next() }
	}

	cfg := cors.DefaultConfig()
	cfg.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	cfg.AllowHeaders = []string{"Origin", "Content-Type", "Authorization", "X-API-Key"}
	cfg.AllowOrigins = []string{m.cfg.Auth.PublicOrigin}
	cfg.AllowCredentials = true
	return cors.New(cfg)
}
