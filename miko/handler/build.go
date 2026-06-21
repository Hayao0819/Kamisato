package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/miko/domain"
	"github.com/Hayao0819/Kamisato/miko/service"
	"github.com/gin-gonic/gin"
)

// SubmitBuildHandler accepts a build request and enqueues it.
// POST /api/unstable/build -> 202 {"job_id": id}
func (h *Handler) SubmitBuildHandler(c *gin.Context) {
	var req domain.BuildRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.WrapErr(err, "invalid build request").Error()})
		return
	}

	id, err := h.s.Submit(&req)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, service.ErrInvalidRequest):
			status = http.StatusBadRequest
		case errors.Is(err, service.ErrQueueFull):
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"job_id": id})
}

// JobStatusHandler returns the current state of a job.
// GET /api/unstable/jobs/:id
func (h *Handler) JobStatusHandler(c *gin.Context) {
	id := c.Param("id")
	job, err := h.s.Status(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, job)
}

// JobListHandler returns all jobs, newest first.
// GET /api/unstable/jobs
func (h *Handler) JobListHandler(c *gin.Context) {
	c.JSON(http.StatusOK, h.s.List())
}

// JobLogsHandler streams the build logs for a job as Server-Sent Events.
// While the job is running it tails the live joblog.Buffer; once the buffer is
// closed the stream ends. If the job has no live buffer it falls back to the
// accumulated text.
// GET /api/unstable/jobs/:id/logs
func (h *Handler) JobLogsHandler(c *gin.Context) {
	id := c.Param("id")
	job, err := h.s.Status(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Fallback: no live buffer, return whatever text we have.
	if job.Log == nil {
		c.String(http.StatusOK, job.Logs)
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	ctx := c.Request.Context()

	// Poll instead of blocking in a goroutine: a goroutine parked in WaitFrom
	// (sync.Cond, no context support) would leak when the client disconnects.
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	offset := 0
	emit := func() bool {
		// Read closed before the buffer so a final write isn't missed.
		closed := job.Log.Closed()
		full := job.Log.String()
		if len(full) > offset {
			chunk := full[offset:]
			lines := strings.Split(chunk, "\n")
			// Drop the empty tail a chunk ending in "\n" produces.
			if n := len(lines); n > 0 && lines[n-1] == "" {
				lines = lines[:n-1]
			}
			for _, line := range lines {
				fmt.Fprintf(c.Writer, "data: %s\n\n", line)
			}
			c.Writer.Flush()
			offset = len(full)
		}
		return closed
	}

	// Flush whatever is already buffered before waiting on the ticker.
	if emit() {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if emit() {
				return
			}
		}
	}
}
