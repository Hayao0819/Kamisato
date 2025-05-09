package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) HelloHandler(ctx *gin.Context) {
	ctx.String(http.StatusOK, "Hello, Ayato!")
}
