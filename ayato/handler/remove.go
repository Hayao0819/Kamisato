package handler

import (
	"fmt"
	"net/http"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/gin-gonic/gin"
)

// BlinkyRemoveHandler is the handler for the package removal API.
func (h *Handler) BlinkyRemoveHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	packageName := ctx.Param("name")
	// The explicit /:repo/:arch/package/:name route scopes the removal to one
	// architecture; the blinky-compatible /:repo/package/:name route carries none,
	// and an empty arch means "remove from every arch that lists the package" (the
	// pkgctl default). For blinky's x86_64-only clients the two coincide.
	archName := ctx.Param("arch")
	if packageName == "" {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "Package name is required"})
		return
	}
	if repoName == "" {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "Repository name is required"})
		return
	}
	if err := h.s.RemovePkg(repoName, archName, packageName); err != nil {
		ctx.JSON(http.StatusInternalServerError, domain.APIError{
			Message: "Remove package file err",
			Reason:  err.Error(),
		})
		return
	}
	ctx.String(http.StatusOK, fmt.Sprintf("'%s' removed!", packageName))
}
