package router

import (
	"time"

	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/ayato/view"
	"github.com/Hayao0819/Kamisato/internal/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// SetRoute sets all API routes for the Ayato server.
func SetRoute(e *gin.Engine, h *handler.Handler, m *middleware.Middleware) error {
	if err := view.Set(e); err != nil {
		return utils.WrapErr(err, "テンプレート設定に失敗")
	}

	// Reverse proxy to miko. miko holds builds/jobs as the single source of truth,
	// and ayato just passes through with an API key (clients never reach miko directly).
	mikoProxy, err := h.MikoProxy()
	if err != nil {
		return utils.WrapErr(err, "miko プロキシの初期化に失敗")
	}

	// authLimit throttles the unauthenticated auth endpoints per client IP.
	// 5 req/s sustained with a burst of 20 is generous for a normal SPA session
	// (a handful of probes) yet tight enough to blunt credential-stuffing and
	// OAuth-state flooding. The callback (driven by GitHub) and logout are
	// excluded. ClientIP() is only trustworthy with trusted_proxies configured.
	authLimit := m.RateLimit(rate.Every(time.Second/5), 20)

	{
		api := e.Group("/api/unstable")
		api.Use(m.Cors())
		{
			api.GET("/hello", h.HelloHandler)
			api.GET("/teapot", h.TeapotHandler)
			api.GET("/repos", h.ReposHandler)
			api.GET("/repos/:repo/archs", h.ArchesHandler)
			api.GET("/:repo/:arch/package", h.AllPkgsHandler)
			api.GET("/:repo/:arch/package/:name", h.PkgDetailHandler) // TODO: not yet implemented (detail fetch)
			api.GET("/:repo/:arch/package/:name/files", h.PkgFilesHandler)
		}

		// Public GitHub-OAuth endpoints (no auth middleware): the login flow,
		// the GitHub callback, the CLI start/exchange, and the session probes.
		{
			api.GET("/auth/github/login", authLimit, h.GitHubLoginHandler)
			api.GET("/auth/github/callback", h.GitHubCallbackHandler)
			api.GET("/auth/cli/start", authLimit, h.CLIStartHandler)
			api.POST("/auth/cli/exchange", authLimit, h.CLIExchangeHandler)
			api.GET("/auth/me", authLimit, h.MeHandler)
			api.POST("/auth/logout", h.LogoutHandler)
		}

		if mikoProxy != nil {
			// Job-status reads are public. The static /jobs can coexist with the existing
			// /:repo param route (ayato already uses both /repos and /:repo).
			api.GET("/jobs", mikoProxy.Handler("/api/unstable/jobs"))
			api.GET("/jobs/:id", mikoProxy.HandlerFunc(func(c *gin.Context) string {
				return "/api/unstable/jobs/" + c.Param("id")
			}))
			api.GET("/jobs/:id/logs", mikoProxy.HandlerFunc(func(c *gin.Context) string {
				return "/api/unstable/jobs/" + c.Param("id") + "/logs"
			}))
			api.GET("/stats", mikoProxy.Handler("/api/unstable/stats"))
		}

		// Blinky-compatible mutating routes accept HTTP Basic where the password
		// field carries a CLI token, so the existing blinky client keeps working.
		blinky := api.Group("")
		{
			blinky.Use(m.RateLimit(rate.Every(time.Second/10), 30), m.RequireAdmin(true))
			blinky.PUT("/:repo/package", h.BlinkyUploadHandler)                // Blinky compatible
			blinky.DELETE("/:repo/package/:name", h.BlinkyRemoveHandler)       // Blinky compatible (arch defaults to x86_64)
			blinky.DELETE("/:repo/:arch/package/:name", h.BlinkyRemoveHandler) // explicit architecture
		}

		// miko proxy mutations require a session or Bearer CLI token (no Basic).
		if mikoProxy != nil {
			authed := api.Group("")
			authed.Use(m.RateLimit(rate.Every(time.Second/10), 30), m.RequireAdmin(false))
			// Build submissions are only accepted behind client authentication.
			authed.POST("/build", mikoProxy.Handler("/api/unstable/build"))
			authed.DELETE("/jobs/:id", mikoProxy.HandlerFunc(func(c *gin.Context) string {
				return "/api/unstable/jobs/" + c.Param("id")
			}))
		}

		// Admin allowlist management (authenticated admin only, no Basic).
		admins := api.Group("/auth/admins")
		{
			admins.Use(m.RateLimit(rate.Every(time.Second/10), 30), m.RequireAdmin(false))
			admins.GET("", h.AdminsListHandler)
			admins.POST("", h.AdminsAddHandler)
			admins.DELETE("/:id", h.AdminsRemoveHandler)
		}
	}
	{
		repo := e.Group("/repo")
		repo.GET("/:repo/:arch", h.RepoFileListHandler)
		repo.GET("/:repo/:arch/:file", h.RepoFileHandler)
	}
	return nil
}
