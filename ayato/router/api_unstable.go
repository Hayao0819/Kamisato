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
		api.GET("/hello", h.HelloHandler)

		auth := api.Group("")
		auth.Use(m.BasicAuth)
		api.PUT("/:repo/package", h.UploadHandler)
		api.DELETE("/:repo/package/:name", h.RemoveHandler)

	}
	{
		repo := e.Group("/repo")
		repo.GET("/", h.RepoListHandler)
		repo.GET("/:repo/:arch", h.RepoFileListHandler)
		repo.GET("/:repo/:arch/:file", h.RepoFileHandler)
	}
}
