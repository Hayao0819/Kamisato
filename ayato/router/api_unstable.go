package router

import (
	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/ayato/view"
	"github.com/gin-gonic/gin"
)

func SetRoute(e *gin.Engine, h *handler.Handler, m *middleware.Middleware) {
	// Set template
	if err := view.Set(e); err != nil {
		panic(e)
	}

	{
		api := e.Group("/api/unstable")
		{
			api.GET("/hello", h.HelloHandler)
			api.GET("/:repo/:arch/package", h.AllPkgsHandler)
			api.GET("/:repo/:arch/package/:name") // TODO: Implement this
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
		repo.GET("/", h.RepoListHandler)
		repo.GET("/:repo/:arch", h.RepoFileListHandler)
		repo.GET("/:repo/:arch/:file", h.RepoFileHandler)
	}
}
