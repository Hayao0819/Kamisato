package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// logTokenTTL is short on purpose: a caller mints the token and immediately opens
// the stream, so it only has to survive that one round trip.
const logTokenTTL = 60 * time.Second

// MintLogTokenHandler issues a short-lived one-time token bound to a job id, so a
// browser EventSource can open the SSE build-log stream without a long-lived
// bearer; it is spent on first use so a leaked stream URL cannot be replayed.
func (h *MikoHandler) MintLogTokenHandler(c *gin.Context) {
	if h.logTokens == nil {
		respondAuthError(c, http.StatusServiceUnavailable, "log tokens not configured")
		return
	}
	token, err := h.logTokens.Mint(c.Param("id"), logTokenTTL)
	if err != nil {
		respondAuthError(c, http.StatusInternalServerError, "token")
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "expires_in": int(logTokenTTL / time.Second)})
}
