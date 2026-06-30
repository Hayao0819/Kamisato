package router

import (
	"github.com/Hayao0819/Kamisato/internal/apikey"
	"github.com/Hayao0819/Kamisato/miko/handler"
	"github.com/gin-gonic/gin"
)

// SetRoute registers the build API under /api/unstable. Routes require an API
// key when configured; with none set the server trusts the closed network
// (ayato is the only caller).
func SetRoute(e *gin.Engine, h *handler.Handler, v *apikey.Verifier) error {
	// Health probes carry no API key, so they live outside the /api/unstable group.
	e.GET("/healthz", func(c *gin.Context) { c.String(200, "ok") })
	e.GET("/readyz", func(c *gin.Context) { c.JSON(200, gin.H{"ready": true}) })

	api := e.Group("/api/unstable")
	api.Use(v.Middleware())
	{
		api.POST("/build", h.SubmitBuildHandler)
		api.GET("/jobs", h.JobListHandler)
		api.GET("/jobs/:id", h.JobStatusHandler)
		api.GET("/jobs/:id/logs", h.JobLogsHandler)
		api.GET("/jobs/:id/artifacts", h.ListArtifactsHandler)
		api.GET("/jobs/:id/artifacts/:name", h.GetArtifactHandler)
		api.DELETE("/jobs/:id", h.JobCancelHandler)
		api.GET("/stats", h.JobStatsHandler)
	}
	return nil
}
