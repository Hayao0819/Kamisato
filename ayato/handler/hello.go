package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) HelloHandler(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"message": "Hello, Ayato!",
	})
}
