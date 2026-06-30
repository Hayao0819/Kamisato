package handler

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// maxSignerKeyBytes bounds the armored worker key a client may submit.
const maxSignerKeyBytes = 1 << 20

// RegisterSignerHandler registers an armored worker public key (request body). The
// service accepts it only if it is certified by a configured master key.
func (h *Handler) RegisterSignerHandler(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxSignerKeyBytes))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body: " + err.Error()})
		return
	}
	fpr, err := h.s.RegisterSigner(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"fingerprint": fpr})
}

func (h *Handler) ListSignersHandler(c *gin.Context) {
	fprs, err := h.s.ListSigners()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"signers": fprs})
}

// UnregisterSignerHandler revokes a registered worker key by fingerprint.
func (h *Handler) UnregisterSignerHandler(c *gin.Context) {
	if err := h.s.UnregisterSigner(c.Param("fingerprint")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "revoked"})
}
