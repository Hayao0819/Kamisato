package handler

import (
	"fmt"
	"net/http"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/gin-gonic/gin"
)

func (h *Handler) BlinkyRemoveHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	packageName := ctx.Param("name")
	// Empty arch (blinky's /:repo/package/:name route has none) means "remove from
	// every arch that lists the package" (pkgctl default); the explicit
	// /:repo/:arch/package/:name route scopes to one arch.
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
		ctx.JSON(errToStatus(err), domain.APIError{
			Message: "Remove package file err",
			Reason:  err.Error(),
		})
		return
	}
	ctx.String(http.StatusOK, fmt.Sprintf("'%s' removed!", packageName))
}
