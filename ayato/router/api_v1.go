package router

import (
	"net/http"

	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/gin-gonic/gin"
)

func SetRoute(e *gin.Engine) {
	api := e.Group("/api/v1")
	{
		api.GET("/", func(c *gin.Context) {
			c.String(http.StatusOK, "Hello, Gin!")
		})
		api.POST("/login", handler.LoginHandler)
		api.POST("/logout", handler.LogoutHandler)
		api.POST("/upload", handler.UploadHandler)
		api.POST("/remove", handler.RemoveHandler)
	}
}
