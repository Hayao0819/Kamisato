package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// logTokenTTL is short on purpose: a caller mints the token and immediately opens
// the stream, so it only has to survive that one round trip.
const logTokenTTL = 60 * time.Second

// MintLogTokenHandler issues a short-lived one-time token bound to a job id. It is
// admin-gated at the router; the token then lets a browser EventSource open the
// SSE build-log stream without carrying a long-lived bearer, and is spent on first
// use so a leaked stream URL cannot be replayed.
func (h *Handler) MintLogTokenHandler(c *gin.Context) {
	if h.logTokens == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "log tokens not configured"})
		return
	}
	token, err := h.logTokens.Mint(c.Param("id"), logTokenTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "expires_in": int(logTokenTTL / time.Second)})
}
