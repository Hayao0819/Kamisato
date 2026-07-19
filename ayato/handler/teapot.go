package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *SystemHandler) TeapotHandler(ctx *gin.Context) {
	ctx.String(http.StatusTeapot, "I'm a teapot!")
}
