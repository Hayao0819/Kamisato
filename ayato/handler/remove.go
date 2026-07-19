package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) BlinkyRemoveHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	packageName := ctx.Param("name")
	// Empty arch (the blinky route has none) means "remove from every arch that lists
	// the package" (pkgctl default); the native arch-scoped route removes from one arch.
	archName := ctx.Param("arch")
	if packageName == "" {
		respondError(ctx, http.StatusBadRequest, "package name is required")
		return
	}
	if repoName == "" {
		respondError(ctx, http.StatusBadRequest, "repository name is required")
		return
	}
	if err := h.s.RemovePkg(repoName, archName, packageName); err != nil {
		respondServiceError(ctx, "remove package", "failed to remove package", err)
		return
	}
	ctx.String(http.StatusOK, fmt.Sprintf("'%s' removed!", packageName))
}
