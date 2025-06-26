package router

import (
	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/ayato/view"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
)

func SetRoute(e *gin.Engine, h handler.IHandler, m *middleware.Middleware) error {
	// Set template
	if err := view.Set(e); err != nil {
		return errors.Wrap(err, "failed to set view")
	}

	{
		api := e.Group("/api/unstable")
		api.Use(m.Cors())
		{
			api.GET("/hello", h.HelloHandler)
			api.GET("/teapot", h.TeapotHandler)
			api.GET("/repos", h.ReposHandler)
			api.GET("/repos/:repo/archs", h.ArchesHandler)
			api.GET("/:repo/:arch/package", h.AllPkgsHandler)
			api.GET("/:repo/:arch/package/:name", h.PkgDetailHandler) // TODO: Implement this
			api.GET("/:repo/:arch/package/:name/files", h.PkgFilesHandler)
		}

		auth := api.Group("")
		{
			auth.Use(m.BasicAuth)
			auth.PUT("/:repo/package", h.BlinkyUploadHandler)          // Blinky compatible
			auth.DELETE("/:repo/package/:name", h.BlinkyRemoveHandler) // Blinky compatible
		}
	}
	{
		repo := e.Group("/repo")
		repo.GET("/:repo/:arch", h.RepoFileListHandler)
		repo.GET("/:repo/:arch/:file", h.RepoFileHandler)
	}
	return nil
}
