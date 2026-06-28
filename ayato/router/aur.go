package router

import (
	"net/http"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/aur"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// Source management is admin-only; the read-only aurweb surface (/rpc,
// /<pkgbase>.git, /cgit/...) is mounted as the NoRoute fallback so it never
// shadows ayato's own routes.
func SetAUR(e *gin.Engine, m *middleware.Middleware, srv http.Handler, h *aur.Handler) {
	// Public kayo-facing catalog + pubkey (no admin creds; kayo verifies the
	// signature instead). Rate-limited because the catalog rebuild lists and
	// unmarshals every registered package, which an attacker could amplify.
	pub := e.Group("/api/unstable/aur")
	pub.Use(m.RateLimit(rate.Every(time.Second/10), 30))
	pub.GET("/catalog", h.CatalogHandler)
	pub.GET("/pubkey", h.PubkeyHandler)

	sources := e.Group("/api/unstable/aur/sources")
	sources.Use(m.RateLimit(rate.Every(time.Second/10), 30), m.RequireAdmin(false))
	sources.GET("", h.ListHandler)
	sources.POST("", h.RegisterHandler)
	sources.DELETE("/:pkgbase", h.RemoveHandler)

	e.NoRoute(gin.WrapH(srv))
}
