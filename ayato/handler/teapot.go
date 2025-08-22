package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// TeapotHandler is a joke API that returns HTTP 418 I'm a teapot.
func (h *Handler) TeapotHandler(ctx *gin.Context) {
	ctx.String(http.StatusTeapot, "I'm a teapot!")
}
