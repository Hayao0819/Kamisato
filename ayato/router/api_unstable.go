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

func SetRoute(e *gin.Engine, h *handler.Handler, m *middleware.Middleware) error {
	if err := view.Set(e); err != nil {
		return utils.WrapErr(err, "failed to configure templates")
	}

	// Reverse proxy to miko. miko holds builds/jobs as the single source of truth,
	// and ayato just passes through with an API key (clients never reach miko directly).
	mikoProxy, err := h.MikoProxy()
	if err != nil {
		return utils.WrapErr(err, "failed to initialize the miko proxy")
	}

	// Throttle unauthenticated auth endpoints per client IP (5 req/s, burst 20)
	// to blunt credential-stuffing and OAuth-state flooding; the callback and
	// logout are excluded. ClientIP() is only trustworthy with trusted_proxies.
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
			api.GET("/:repo/:arch/package/:name", h.PkgDetailHandler)
			api.GET("/:repo/:arch/package/:name/files", h.PkgFilesHandler)
			api.GET("/:repo/:arch/signed-url", h.SignedURLHandler)

			// Advertises which optional features are configured so the UI hides what
			// is unavailable (bug reporting, miko build views, GitHub login).
			api.GET("/features", h.FeaturesHandler)
			// POST forwards a bug report; rate-limited because it opens an issue on
			// the upstream tracker.
			api.POST("/bug-reports", m.RateLimit(rate.Every(2*time.Second), 3), h.SubmitBugReportHandler)
		}

		// Public GitHub-OAuth endpoints (no auth middleware).
		{
			api.GET("/auth/github/login", authLimit, h.GitHubLoginHandler)
			api.GET("/auth/github/callback", h.GitHubCallbackHandler)
			api.GET("/auth/cli/start", authLimit, h.CLIStartHandler)
			api.POST("/auth/cli/exchange", authLimit, h.CLIExchangeHandler)
			api.GET("/auth/web/start", authLimit, h.WebStartHandler)
			api.POST("/auth/web/exchange", authLimit, h.WebExchangeHandler)
			api.GET("/auth/me", authLimit, h.MeHandler)
			api.POST("/auth/logout", h.LogoutHandler)
		}

		if mikoProxy != nil {
			// Static /jobs coexists with the /:repo param route (ayato already uses
			// both /repos and /:repo).
			api.GET("/jobs", mikoProxy.Handler("/api/unstable/jobs"))
			api.GET("/jobs/:id", mikoProxy.HandlerFunc(func(c *gin.Context) string {
				return "/api/unstable/jobs/" + c.Param("id")
			}))
			api.GET("/jobs/:id/logs", mikoProxy.HandlerFunc(func(c *gin.Context) string {
				return "/api/unstable/jobs/" + c.Param("id") + "/logs"
			}))
			api.GET("/jobs/:id/artifacts", mikoProxy.HandlerFunc(func(c *gin.Context) string {
				return "/api/unstable/jobs/" + c.Param("id") + "/artifacts"
			}))
			api.GET("/jobs/:id/artifacts/:name", mikoProxy.HandlerFunc(func(c *gin.Context) string {
				return "/api/unstable/jobs/" + c.Param("id") + "/artifacts/" + c.Param("name")
			}))
			api.GET("/stats", mikoProxy.Handler("/api/unstable/stats"))
		}

		// Upload accepts an admin user (Basic password = CLI token) or a CI
		// publisher (API key / GitHub OIDC). Removal stays admin-only.
		upload := api.Group("")
		{
			upload.Use(m.RateLimit(rate.Every(time.Second/10), 30), m.RequireUpload())
			upload.PUT("/:repo/package", h.BlinkyUploadHandler)  // Blinky compatible (single)
			upload.POST("/:repo/packages", h.BatchUploadHandler) // atomic multi-package publish
		}
		blinky := api.Group("")
		{
			blinky.Use(m.RateLimit(rate.Every(time.Second/10), 30), m.RequireAdmin(true))
			blinky.DELETE("/:repo/package/:name", h.BlinkyRemoveHandler)       // Blinky compatible (arch defaults to x86_64)
			blinky.DELETE("/:repo/:arch/package/:name", h.BlinkyRemoveHandler) // explicit architecture
		}

		// miko proxy mutations require a session or Bearer CLI token (no Basic).
		if mikoProxy != nil {
			authed := api.Group("")
			authed.Use(m.RateLimit(rate.Every(time.Second/10), 30), m.RequireAdmin(false))
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

		// Worker signing-key registration. RequireAdmin(true) allows a Basic CLI
		// token so miko can self-register at boot; the master-certification chain
		// is the real gate (only master-certified worker keys are stored).
		signers := api.Group("/auth/signers")
		{
			signers.Use(m.RateLimit(rate.Every(time.Second/10), 30), m.RequireAdmin(true))
			signers.GET("", h.ListSignersHandler)
			signers.POST("", h.RegisterSignerHandler)
			signers.DELETE("/:fingerprint", h.UnregisterSignerHandler)
		}
	}
	{
		repo := e.Group("/repo")
		repo.GET("/:repo/:arch", h.RepoFileListHandler)
		repo.GET("/:repo/:arch/:file", h.RepoFileHandler)
	}
	return nil
}
