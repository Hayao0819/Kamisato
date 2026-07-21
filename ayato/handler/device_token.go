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
		respondAuthError(c, http.StatusServiceUnavailable, "device login not configured")
		return
	}
	var body struct {
		DeviceCode string `json:"device_code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondAuthError(c, http.StatusBadRequest, "invalid request")
		return
	}

	if h.replay != nil {
		firstUse, err := h.replay.Consume("devpoll:"+body.DeviceCode, deviceInterval)
		if err == nil && !firstUse {
			respondAuthError(c, http.StatusBadRequest, "slow_down")
			return
		}
	}

	status, githubID, login, ok, err := h.device.PollDevice(body.DeviceCode)
	if err != nil {
		respondAuthError(c, http.StatusInternalServerError, "device store")
		return
	}
	if !ok {
		respondAuthError(c, http.StatusBadRequest, "expired_token")
		return
	}
	switch status {
	case auth.DeviceApproved:
		h.issueDeviceToken(c, body.DeviceCode, githubID, login)
	case auth.DeviceDenied:
		respondAuthError(c, http.StatusBadRequest, "access_denied")
	default:
		respondAuthError(c, http.StatusBadRequest, "authorization_pending")
	}
}

func (h *AuthHandler) issueDeviceToken(
	c *gin.Context,
	deviceCode string,
	githubID int64,
	login string,
) {
	if !h.admins.IsAdmin(githubID) {
		respondAuthError(c, http.StatusBadRequest, "access_denied")
		return
	}
	access, refresh, expiresIn, err := h.issueAccessRefresh(githubID, login)
	if err != nil {
		respondAuthError(c, http.StatusInternalServerError, "token")
		return
	}
	consumed, err := h.device.ConsumeDevice(deviceCode)
	if err != nil {
		respondAuthError(c, http.StatusServiceUnavailable, "device store")
		return
	}
	if !consumed {
		respondAuthError(c, http.StatusBadRequest, "expired_token")
		return
	}
	respondTokenPair(c, access, refresh, expiresIn, login, githubID)
}
