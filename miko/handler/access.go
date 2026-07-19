package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/internal/auth/apikey"
	"github.com/Hayao0819/Kamisato/miko/domain"
)

// RequireJobAccess limits non-admin principals to their own jobs.
func (h *Handler) RequireJobAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, authenticated := apikey.PrincipalFrom(c)
		if !authenticated || principal.Allows(apikey.ScopeBuildAdmin) {
			c.Next()
			return
		}
		job, err := h.s.Status(c.Param("id"))
		if err != nil || job.Owner == "" || job.Owner != principal.Name {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.Next()
	}
}

func visibleJobs(c *gin.Context, jobs []*domain.BuildJob) []*domain.BuildJob {
	principal, authenticated := apikey.PrincipalFrom(c)
	if !authenticated || principal.Allows(apikey.ScopeBuildAdmin) {
		return jobs
	}
	visible := make([]*domain.BuildJob, 0, len(jobs))
	for _, job := range jobs {
		if job.Owner == principal.Name {
			visible = append(visible, job)
		}
	}
	return visible
}
