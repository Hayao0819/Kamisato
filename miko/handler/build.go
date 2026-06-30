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

// GET /api/unstable/jobs
func (h *Handler) JobListHandler(c *gin.Context) {
	c.JSON(http.StatusOK, h.s.List())
}

// DELETE /api/unstable/jobs/:id
func (h *Handler) JobCancelHandler(c *gin.Context) {
	id := c.Param("id")
	if err := h.s.Cancel(id); err != nil {
		status := http.StatusNotFound
		if errors.Is(err, service.ErrJobNotCancelable) {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

// GET /api/unstable/stats
func (h *Handler) JobStatsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, h.s.Stats())
}

// JobLogsHandler streams a job's build logs as Server-Sent Events, tailing the
// live buffer and falling back to accumulated text when there is none.
// GET /api/unstable/jobs/:id/logs
func (h *Handler) JobLogsHandler(c *gin.Context) {
	id := c.Param("id")
	job, err := h.s.Status(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Cap concurrent SSE readers per job to bound long-lived streaming goroutines.
	maxReaders := 8
	if h.cfg != nil && h.cfg.MaxLogReaders > 0 {
		maxReaders = h.cfg.MaxLogReaders
	}
	h.logReadersMu.Lock()
	if h.logReaders[id] >= maxReaders {
		h.logReadersMu.Unlock()
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many concurrent log readers for this job"})
		return
	}
	h.logReaders[id]++
	h.logReadersMu.Unlock()
	defer func() {
		h.logReadersMu.Lock()
		if h.logReaders[id] <= 1 {
			delete(h.logReaders, id)
		} else {
			h.logReaders[id]--
		}
		h.logReadersMu.Unlock()
	}()

	// Fallback: no live buffer, return whatever text we have.
	if job.Log == nil {
		c.String(http.StatusOK, job.Logs)
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	ctx := c.Request.Context()

	// Per-flush write deadline: a stuck client makes Flush error instead of pinning this goroutine.
	rc := http.NewResponseController(c.Writer)
	const flushDeadline = 30 * time.Second

	// Poll instead of parking a goroutine on a condition variable (no context
	// support), which would leak when the client disconnects.
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	offset := 0
	emit := func() bool {
		// BytesFrom reads closed atomically with the bytes, so the final write isn't missed.
		chunk, total, closed := job.Log.BytesFrom(offset)
		if len(chunk) > 0 {
			lines := strings.Split(string(chunk), "\n")
			// Drop the empty tail a chunk ending in "\n" produces.
			if n := len(lines); n > 0 && lines[n-1] == "" {
				lines = lines[:n-1]
			}
			for _, line := range lines {
				fmt.Fprintf(c.Writer, "data: %s\n\n", line)
			}
			_ = rc.SetWriteDeadline(time.Now().Add(flushDeadline))
			c.Writer.Flush()
			offset = total
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
