package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AuthRequiredResponse represents the response for the auth requirement endpoint.
type AuthRequiredResponse struct {
	Required bool `json:"required"`
}

// AuthRequiredHandler returns whether authentication is required for uploads.
func (h *Handler) AuthRequiredHandler(ctx *gin.Context) {
	// Authentication is required if either username or password is set
	required := h.cfg.Auth.Username != "" || h.cfg.Auth.Password != ""

	ctx.JSON(http.StatusOK, AuthRequiredResponse{
		Required: required,
	})
}
