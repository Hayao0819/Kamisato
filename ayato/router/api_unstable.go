package router

import (
	"time"

	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/ayato/router/view"
	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func SetRoute(e *gin.Engine, h *handler.Handler, m *middleware.Middleware) error {
	if err := view.Set(e); err != nil {
		return errors.WrapErr(err, "failed to configure templates")
	}

	// Health/readiness probes carry no auth and sit outside /api/unstable so
	// orchestrators (Cloud Run, Kubernetes) can reach them without credentials.
	e.GET("/health", func(c *gin.Context) { c.String(200, "ok") })
	e.GET("/ready", func(c *gin.Context) { c.JSON(200, gin.H{"ready": true}) })

	// Reverse proxy to miko. miko holds builds/jobs as the single source of truth,
	// and ayato just passes through with an API key (clients never reach miko directly).
	mikoProxy, err := h.MikoProxy()
	if err != nil {
		return errors.WrapErr(err, "failed to initialize the miko proxy")
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
			api.GET("/repos/:repo", h.RepoDetailHandler)
			api.GET("/repos/:repo/:arch/packages", h.AllPkgsHandler)
			api.GET("/repos/:repo/:arch/packages/:name", h.PkgDetailHandler)
			api.GET("/repos/:repo/:arch/packages/:name/files", h.PkgFilesHandler)
			api.GET("/repos/:repo/:arch/signed-url", h.SignedURLHandler)

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
			// Device authorization (RFC 8628) for browserless CLI login.
			api.POST("/auth/device/code", authLimit, h.DeviceCodeHandler)
			api.GET("/auth/device", authLimit, h.DeviceVerifyHandler)
			api.GET("/auth/device/approve", authLimit, h.DeviceApproveHandler)
			api.POST("/auth/device/token", authLimit, h.DeviceTokenHandler)
			api.GET("/auth/me", authLimit, h.MeHandler)
			api.POST("/auth/logout", h.LogoutHandler)
			// Revokes the presented CLI token by its jti; the signature is the auth.
			api.POST("/auth/cli/revoke", authLimit, h.RevokeCLIHandler)
			// Trades a valid refresh token for a fresh short-lived access token.
			api.POST("/auth/refresh", authLimit, h.RefreshHandler)
		}

		if mikoProxy != nil {
			// Job metadata/artifacts/stats reads are admin-gated below. Live build-log
			// streaming is browser-facing over EventSource, which cannot attach a Bearer;
			// RequireLogAccess therefore accepts a one-time token (query "token", bound to
			// this job, spent on first use) as an alternative to admin session/bearer. This
			// closes the formerly-public hole — a build log can echo credentials from a
			// user-supplied git URL.
			api.GET("/jobs/:id/logs", m.RequireLogAccess(), mikoProxy.HandlerFunc(func(c *gin.Context) string {
				return "/api/unstable/jobs/" + c.Param("id") + "/logs"
			}))
		}

		// Native publish. Accepts a CI publisher (API key / GitHub OIDC) scoped to the
		// repo, or an admin. The blinky-compatible single upload lives under /blinky.
		upload := api.Group("")
		{
			upload.Use(m.RateLimit(rate.Every(time.Second/10), 30), m.RequireCI())
			upload.POST("/repos/:repo/packages", h.BatchUploadHandler) // atomic multi-package publish
			// Direct-to-R2 upload for packages too large for the request-body limit:
			// presign a PUT, the client uploads straight to the store, then finalize
			// validates and registers the already-stored object.
			upload.POST("/repos/:repo/packages/presign", h.PresignUploadHandler)
			upload.POST("/repos/:repo/packages/finalize", h.FinalizeUploadHandler)
		}
		// Package removal is publish-scoped like upload: a CI publisher may prune its
		// own repos (deleting a package whose PKGBUILD is gone is part of managing the
		// repo), or an admin may remove.
		remove := api.Group("")
		{
			remove.Use(m.RateLimit(rate.Every(time.Second/10), 30), m.RequireCI())
			remove.DELETE("/repos/:repo/:arch/packages/:name", h.BlinkyRemoveHandler)
		}
		// Native repo management (admin-only).
		mgmt := api.Group("")
		{
			mgmt.Use(m.RateLimit(rate.Every(time.Second/10), 30), m.RequireAdmin())
			// Tier promotion advances a package through a tiered repo's
			// staging -> testing -> stable flow; a release action, so admin-gated.
			mgmt.POST("/repos/:repo/promote", h.PromoteHandler)
			// Refresh an upstream-layered repo's merged database from its upstream.
			mgmt.POST("/repos/:repo/sync-upstream", h.SyncUpstreamHandler)
		}

		// miko proxy reads and mutations require a session or Bearer CLI token (no
		// Basic). Job metadata, artifacts, and stats are admin-only, matching the
		// build mutation; /jobs/:id/logs takes a one-time token or admin (see above).
		if mikoProxy != nil {
			authed := api.Group("")
			authed.Use(m.RateLimit(rate.Every(time.Second/10), 30), m.RequireAdmin())
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
			admins.Use(m.RateLimit(rate.Every(time.Second/10), 30), m.RequireAdmin())
			admins.GET("", h.AdminsListHandler)
			admins.POST("", h.AdminsAddHandler)
			admins.DELETE("/:id", h.AdminsRemoveHandler)
		}

		// Worker signing-key registration. RequireBlinkyAdmin allows a Basic CLI token
		// so miko can self-register at boot; the master-certification chain is the real
		// gate (only master-certified worker keys are stored).
		signers := api.Group("/auth/signers")
		{
			signers.Use(m.RateLimit(rate.Every(time.Second/10), 30), m.RequireBlinkyAdmin())
			signers.GET("", h.ListSignersHandler)
			signers.POST("", h.RegisterSignerHandler)
			signers.DELETE("/:fingerprint", h.UnregisterSignerHandler)
		}
	}
	{
		// Blinky compatibility surface. The blinky clientlib builds
		// <base>/api/unstable/<repo>/package, so a client configured with
		// `blinky login https://host/blinky` reaches these routes verbatim. Basic
		// auth only, matching blinky (native publish is /api/unstable/repos/...).
		blinky := e.Group("/blinky/api/unstable")
		blinky.Use(m.RateLimit(rate.Every(time.Second/10), 30), m.RequireBlinkyAdmin())
		blinky.PUT("/:repo/package", h.BlinkyUploadHandler)
		blinky.DELETE("/:repo/package/:name", h.BlinkyRemoveHandler)
	}
	{
		repo := e.Group("/repo")
		if err := view.SetRepoAssets(repo); err != nil {
			return errors.WrapErr(err, "failed to register repo index assets")
		}
		// Static: takes priority over the :arch route below, so no repo may serve
		// an architecture literally named "mirrorlist".
		repo.GET("/:repo/mirrorlist", h.MirrorlistHandler)
		repo.GET("/:repo/:arch", h.RepoFileListHandler)
		repo.GET("/:repo/:arch/:file", h.RepoFileHandler)
	}
	return nil
}
