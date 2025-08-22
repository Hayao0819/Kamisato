package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HelloHandler is an endpoint for API connectivity check.
func (h *Handler) HelloHandler(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"message": "Hello, Ayato!",
	})
}
