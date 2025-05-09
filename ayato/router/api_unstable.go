package router

import (
	"net/http"

	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/gin-gonic/gin"
)

func SetRoute(e *gin.Engine) {
	api := e.Group("/api/unstable")
	api.Use(middleware.Config())

	api.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Hello, Ayato.")
	})

	api.GET("/info")

	{
		auth := api.Group("")
		auth.Use(middleware.BasicAuth())
		api.PUT("/:repo/package/:name", handler.UploadHandler)
		api.DELETE("/:repo/package/:name", handler.RemoveHandler)
	}
}
