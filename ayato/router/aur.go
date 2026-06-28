package router

import (
	"net/http"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/aur"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// SetAUR wires the aurweb-compatible surface onto the engine. Source management
// is admin-only; the read-only aurweb surface (/rpc, /<pkgbase>.git, /cgit/...)
// is mounted as the root fallback so it never shadows ayato's own routes.
func SetAUR(e *gin.Engine, m *middleware.Middleware, srv http.Handler, h *aur.Handler) {
	// Public sara-facing catalog + signing public key (no admin credentials;
	// sara verifies the catalog's signature instead).
	e.GET("/api/unstable/aur/catalog", h.CatalogHandler)
	e.GET("/api/unstable/aur/pubkey", h.PubkeyHandler)

	sources := e.Group("/api/unstable/aur/sources")
	sources.Use(m.RateLimit(rate.Every(time.Second/10), 30), m.RequireAdmin(false))
	sources.GET("", h.ListHandler)
	sources.POST("", h.RegisterHandler)
	sources.DELETE("/:pkgbase", h.RemoveHandler)

	e.NoRoute(gin.WrapH(srv))
}
