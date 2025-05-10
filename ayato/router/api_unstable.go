package router

import (
	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/conf"
	"github.com/gin-gonic/gin"
)

func SetRoute(e *gin.Engine, cfg *conf.AyatoConfig, kv repository.Repository) {

	h := handler.NewHandler(cfg, kv)
	m := middleware.NewMiddleware(cfg)

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
		repo.GET("/:repo", h.RepoHandler)
	}
}
