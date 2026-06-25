package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Cors is a Gin middleware for CORS settings.
//
// /api is served same-origin through the lumine BFF, and the CLI calls ayato
// directly (CORS does not apply to non-browser clients), so a permissive
// AllowAllOrigins is both unnecessary and unsafe once cookies are in play
// (wildcard origin + credentials is forbidden by the spec and a CSRF risk).
//
// When PublicOrigin is configured we allow exactly that origin with
// credentials; otherwise we fall back to a no-origin, no-credentials policy
// that still permits public reads (the SPA itself is same-origin so it never
// triggers a CORS preflight).
func (m *Middleware) Cors() gin.HandlerFunc {
	if m.cfg == nil || m.cfg.Auth.PublicOrigin == "" {
		// No cross-origin browser access is expected (the SPA is same-origin
		// behind lumine, the CLI is non-browser). Emit no CORS headers rather
		// than echoing arbitrary origins. Same-origin and non-CORS requests are
		// unaffected; genuine cross-origin browser requests get no
		// Access-Control-Allow-Origin and are blocked by the browser.
		return func(c *gin.Context) { c.Next() }
	}

	cfg := cors.DefaultConfig()
	cfg.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	cfg.AllowHeaders = []string{"Origin", "Content-Type", "Authorization", "X-API-Key"}
	cfg.AllowOrigins = []string{m.cfg.Auth.PublicOrigin}
	cfg.AllowCredentials = true
	return cors.New(cfg)
}
