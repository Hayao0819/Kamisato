package router

import (
	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/ayato/view"
	"github.com/Hayao0819/Kamisato/internal/utils"

	"github.com/gin-gonic/gin"
)

// SetRoute sets all API routes for the Ayato server.
// It configures all API routes for the Ayato server.
func SetRoute(e *gin.Engine, h *handler.Handler, m *middleware.Middleware) error {
	// Template setup
	if err := view.Set(e); err != nil {
		return utils.WrapErr(err, "テンプレート設定に失敗")
	}

	// Reverse proxy to miko. miko holds builds/jobs as the single source of truth,
	// and ayato just passes through with an API key (clients never reach miko directly).
	mikoProxy, err := h.MikoProxy()
	if err != nil {
		return utils.WrapErr(err, "miko プロキシの初期化に失敗")
	}

	{
		api := e.Group("/api/unstable")
		api.Use(m.Cors())
		{
			api.GET("/hello", h.HelloHandler)
			api.GET("/teapot", h.TeapotHandler)
			api.GET("/auth/required", h.AuthRequiredHandler)
			api.GET("/repos", h.ReposHandler)
			api.GET("/repos/:repo/archs", h.ArchesHandler)
			api.GET("/:repo/:arch/package", h.AllPkgsHandler)
			api.GET("/:repo/:arch/package/:name", h.PkgDetailHandler) // TODO: not yet implemented (detail fetch)
			api.GET("/:repo/:arch/package/:name/files", h.PkgFilesHandler)
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
		}

		auth := api.Group("")
		{
			auth.Use(m.BasicAuth)
			auth.PUT("/:repo/package", h.BlinkyUploadHandler)          // Blinky compatible
			auth.DELETE("/:repo/package/:name", h.BlinkyRemoveHandler) // Blinky compatible
			if mikoProxy != nil {
				// Build submissions are only accepted behind client authentication.
				auth.POST("/build", mikoProxy.Handler("/api/unstable/build"))
			}
		}
	}
	{
		repo := e.Group("/repo")
		repo.GET("/:repo/:arch", h.RepoFileListHandler)
		repo.GET("/:repo/:arch/:file", h.RepoFileHandler)
	}
	return nil
}
