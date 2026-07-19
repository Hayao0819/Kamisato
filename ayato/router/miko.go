package router

import (
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
)

func setMikoRoutes(
	api *gin.RouterGroup,
	miko *handler.MikoHandler,
	middlewares *middleware.Middleware,
	proxy *handler.MikoProxy,
) {
	if proxy == nil {
		return
	}

	// EventSource cannot attach a bearer token, so log streaming also accepts a
	// one-time job-bound token through RequireLogAccess.
	api.GET(
		"/jobs/:id/logs",
		middlewares.RequireLogAccess(),
		proxy.HandlerFunc(func(c *gin.Context) []string {
			return []string{"api", "unstable", "jobs", c.Param("id"), "logs"}
		}),
	)

	authed := api.Group("")
	authed.Use(
		middlewares.RateLimit(rate.Every(time.Second/10), 30),
		middlewares.RequireAdmin(),
	)
	authed.POST("/jobs/:id/logs/token", miko.MintLogTokenHandler)
	authed.GET("/jobs", proxy.Handler("api", "unstable", "jobs"))
	authed.GET("/jobs/:id", proxy.HandlerFunc(func(c *gin.Context) []string {
		return []string{"api", "unstable", "jobs", c.Param("id")}
	}))
	authed.GET("/jobs/:id/artifacts", proxy.HandlerFunc(func(c *gin.Context) []string {
		return []string{"api", "unstable", "jobs", c.Param("id"), "artifacts"}
	}))
	authed.GET("/jobs/:id/artifacts/:name", proxy.HandlerFunc(func(c *gin.Context) []string {
		return []string{"api", "unstable", "jobs", c.Param("id"), "artifacts", c.Param("name")}
	}))
	authed.GET("/stats", proxy.Handler("api", "unstable", "stats"))
	authed.POST("/build", proxy.Handler("api", "unstable", "build"))
	authed.DELETE("/jobs/:id", proxy.HandlerFunc(func(c *gin.Context) []string {
		return []string{"api", "unstable", "jobs", c.Param("id")}
	}))
}
