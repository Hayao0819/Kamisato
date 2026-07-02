package router

import (
	"time"

	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/ayato/view"
	"github.com/Hayao0819/Kamisato/internal/errwrap"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func SetRoute(e *gin.Engine, h *handler.Handler, m *middleware.Middleware) error {
	if err := view.Set(e); err != nil {
		return errwrap.WrapErr(err, "failed to configure templates")
	}

	// Reverse proxy to miko. miko holds builds/jobs as the single source of truth,
	// and ayato just passes through with an API key (clients never reach miko directly).
	mikoProxy, err := h.MikoProxy()
	if err != nil {
		return errwrap.WrapErr(err, "failed to initialize the miko proxy")
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
			// Revokes the presented CLI token by its jti; the signature is the auth.
			api.POST("/auth/cli/revoke", authLimit, h.RevokeCLIHandler)
		}

		if mikoProxy != nil {
			// The job metadata/artifacts/stats reads are admin-gated below. Live
			// build-log streaming is browser-facing over EventSource, which cannot
			// attach a Bearer token; RequireLogAccess therefore accepts a one-time
			// token (query "token", minted at .../logs/token and bound to this job)
			// as an alternative to an admin session/bearer, spent on first use. This
			// closes the formerly-public hole — a build log can echo credentials from
			// a user-supplied git URL.
			api.GET("/jobs/:id/logs", m.RequireLogAccess(), mikoProxy.HandlerFunc(func(c *gin.Context) string {
				return "/api/unstable/jobs/" + c.Param("id") + "/logs"
			}))
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

		// miko proxy reads and mutations require a session or Bearer CLI token (no
		// Basic). Job metadata, artifacts, and stats are admin-only, matching the
		// build mutation; /jobs/:id/logs takes a one-time token or admin (see above).
		if mikoProxy != nil {
			authed := api.Group("")
			authed.Use(m.RateLimit(rate.Every(time.Second/10), 30), m.RequireAdmin(false))
			// Mints a one-time token bound to the job so a browser EventSource can
			// open /jobs/:id/logs without a long-lived bearer.
			authed.POST("/jobs/:id/logs/token", h.MintLogTokenHandler)
			// Static /jobs coexists with the /:repo param route (ayato already uses
			// both /repos and /:repo).
			authed.GET("/jobs", mikoProxy.Handler("/api/unstable/jobs"))
			authed.GET("/jobs/:id", mikoProxy.HandlerFunc(func(c *gin.Context) string {
				return "/api/unstable/jobs/" + c.Param("id")
			}))
			authed.GET("/jobs/:id/artifacts", mikoProxy.HandlerFunc(func(c *gin.Context) string {
				return "/api/unstable/jobs/" + c.Param("id") + "/artifacts"
			}))
			authed.GET("/jobs/:id/artifacts/:name", mikoProxy.HandlerFunc(func(c *gin.Context) string {
				return "/api/unstable/jobs/" + c.Param("id") + "/artifacts/" + c.Param("name")
			}))
			authed.GET("/stats", mikoProxy.Handler("/api/unstable/stats"))
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
		if err := view.SetRepoAssets(repo); err != nil {
			return errwrap.WrapErr(err, "failed to register repo index assets")
		}
		repo.GET("/:repo/:arch", h.RepoFileListHandler)
		repo.GET("/:repo/:arch/:file", h.RepoFileHandler)
	}
	return nil
}
