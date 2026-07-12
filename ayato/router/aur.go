package router

import (
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
)

// Source management is admin-only; the read-only aurweb surface (/rpc,
// /<pkgbase>.git, /cgit/...) is mounted as the NoRoute fallback so it never
// shadows ayato's own routes.
func SetAUR(e *gin.Engine, m *middleware.Middleware, srv http.Handler, h *handler.AURHandler) {
	// Public kayo-facing catalog + pubkey (no admin creds; kayo verifies the
	// signature instead). Rate-limited because the catalog rebuild lists and
	// unmarshals every registered package, which an attacker could amplify.
	pub := e.Group("/api/unstable/aur")
	pub.Use(m.RateLimit(rate.Every(time.Second/10), 30))
	pub.GET("/catalog", h.CatalogHandler)
	pub.GET("/pubkey", h.PubkeyHandler)

	sources := e.Group("/api/unstable/aur/sources")
	sources.Use(m.RateLimit(rate.Every(time.Second/10), 30), m.RequireAdmin())
	sources.GET("", h.ListHandler)
	sources.POST("", h.RegisterHandler)
	sources.DELETE("/:pkgbase", h.RemoveHandler)

	// The aurweb NoRoute surface (/rpc, /<pkgbase>.git, cgit) is unauthenticated
	// and drives the same catalog/git machinery, so rate-limit it per client IP as
	// well. A dedicated limiter (not the /api/unstable/aur group's) keeps the two
	// from sharing buckets; the group's routes are matched, so they never fall
	// through to NoRoute and are never double-limited.
	aurLimit := m.RateLimit(rate.Every(time.Second/10), 30)
	e.NoRoute(func(c *gin.Context) {
		aurLimit(c)
		if c.IsAborted() {
			return
		}
		// The aurweb /rpc limiter keys on Request.RemoteAddr; rewrite it to gin's
		// trusted-proxy-aware ClientIP so it counts the real client (not the
		// fronting proxy that all requests would otherwise share) while still
		// ignoring a spoofable X-Forwarded-For when no trusted_proxies are set.
		c.Request.RemoteAddr = net.JoinHostPort(c.ClientIP(), "0")
		srv.ServeHTTP(c.Writer, c.Request)
	})
}
