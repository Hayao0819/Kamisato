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
	archName := "x86_64"
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
			Reason:  err,
		})
		return
	}

	ctx.String(http.StatusOK, fmt.Sprintf("'%s' removed!", packageName))
}
