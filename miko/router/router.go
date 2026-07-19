package router

import (
	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/internal/auth/apikey"
	"github.com/Hayao0819/Kamisato/miko/handler"
)

// SetRoute registers the build API under /api/unstable. Routes require an API
// key when configured. The root command permits no verifier only when the
// operator explicitly set allow_unauthenticated for an isolated dev network.
func SetRoute(e *gin.Engine, h *handler.Handler, v *apikey.Verifier) error {
	// Health probes carry no API key, so they live outside the /api/unstable group.
	e.GET("/health", func(c *gin.Context) { c.String(200, "ok") })
	e.GET("/ready", func(c *gin.Context) { c.JSON(200, gin.H{"ready": true}) })

	api := e.Group("/api/unstable")
	{
		api.POST("/build", v.Middleware(apikey.ScopeBuildSubmit), h.SubmitBuildHandler)
		api.GET("/jobs", v.Middleware(apikey.ScopeBuildRead), h.JobListHandler)
		api.GET("/jobs/:id", v.Middleware(apikey.ScopeBuildRead), h.RequireJobAccess(), h.JobStatusHandler)
		api.GET("/jobs/:id/logs", v.Middleware(apikey.ScopeBuildRead), h.RequireJobAccess(), h.JobLogsHandler)
		api.GET("/jobs/:id/artifacts", v.Middleware(apikey.ScopeBuildRead), h.RequireJobAccess(), h.ListArtifactsHandler)
		api.GET("/jobs/:id/artifacts/:name", v.Middleware(apikey.ScopeBuildRead), h.RequireJobAccess(), h.GetArtifactHandler)
		api.DELETE("/jobs/:id", v.Middleware(apikey.ScopeBuildCancel), h.RequireJobAccess(), h.JobCancelHandler)
		api.GET("/stats", v.Middleware(apikey.ScopeBuildAdmin), h.JobStatsHandler)
	}
	return nil
}
