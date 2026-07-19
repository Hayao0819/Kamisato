package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth"
)

// DeviceTokenHandler is the RFC 8628 §3.4 polling endpoint. Once approved it
// mints the CLI token and consumes the device_code so it cannot be redeemed
// twice.
func (h *AuthHandler) DeviceTokenHandler(c *gin.Context) {
	if h.signer == nil || h.device == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "device login not configured"})
		return
	}
	var body struct {
		DeviceCode string `json:"device_code"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.DeviceCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if h.replay != nil {
		firstUse, err := h.replay.Consume("devpoll:"+body.DeviceCode, deviceInterval)
		if err == nil && !firstUse {
			c.JSON(http.StatusBadRequest, gin.H{"error": "slow_down"})
			return
		}
	}

	status, githubID, login, ok, err := h.device.PollDevice(body.DeviceCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "device store"})
		return
	}
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expired_token"})
		return
	}
	switch status {
	case auth.DeviceApproved:
		h.issueDeviceToken(c, body.DeviceCode, githubID, login)
	case auth.DeviceDenied:
		c.JSON(http.StatusBadRequest, gin.H{"error": "access_denied"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "authorization_pending"})
	}
}

func (h *AuthHandler) issueDeviceToken(
	c *gin.Context,
	deviceCode string,
	githubID int64,
	login string,
) {
	if !h.admins.IsAdmin(githubID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "access_denied"})
		return
	}
	access, refresh, expiresIn, err := h.issueAccessRefresh(githubID, login, "cli")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token"})
		return
	}
	consumed, err := h.device.ConsumeDevice(deviceCode)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "device store"})
		return
	}
	if !consumed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expired_token"})
		return
	}
	respondTokenPair(c, access, refresh, expiresIn, login, githubID)
}
