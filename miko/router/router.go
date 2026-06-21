package router

import (
	"github.com/Hayao0819/Kamisato/internal/apikey"
	"github.com/Hayao0819/Kamisato/miko/handler"
	"github.com/gin-gonic/gin"
)

// SetRoute registers the miko build API routes under /api/unstable. Routes
// require a valid API key when one is configured; with none set the server
// trusts the closed network. ayato is the only caller.
func SetRoute(e *gin.Engine, h *handler.Handler, v *apikey.Verifier) error {
	api := e.Group("/api/unstable")
	api.Use(v.Middleware())
	{
		api.POST("/build", h.SubmitBuildHandler)
		api.GET("/jobs", h.JobListHandler)
		api.GET("/jobs/:id", h.JobStatusHandler)
		api.GET("/jobs/:id/logs", h.JobLogsHandler)
	}
	return nil
}
