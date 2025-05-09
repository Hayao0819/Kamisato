package router

import (
	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/conf"
	"github.com/gin-gonic/gin"
)

func SetRoute(e *gin.Engine, cfg *conf.AyatoConfig) {

	h := handler.NewHandler(cfg)

	{
		api := e.Group("/api/unstable")
		api.Use(middleware.Config())
		api.GET("/", h.HelloHandler)

		auth := api.Group("")
		auth.Use(middleware.BasicAuth())
		api.PUT("/:repo/package/:name", h.UploadHandler)
		api.DELETE("/:repo/package/:name", h.RemoveHandler)

	}
	{
		repo := e.Group("/repo")
		repo.Use(middleware.Config())
		repo.GET("/:repo", h.RepoHandler)
	}
}
