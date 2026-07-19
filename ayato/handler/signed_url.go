package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *RepositoryHandler) SignedURLHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	arch := ctx.Param("arch")
	name := ctx.Query("name")

	if repoName == "" || arch == "" || name == "" {
		respondError(ctx, http.StatusBadRequest, "repository name, architecture, and file name are required")
		return
	}

	url, err := h.reader.SignedURL(repoName, arch, name)
	if url == "" && err == nil {
		ctx.Status(http.StatusNoContent)
		return
	}
	if err != nil {
		respondServiceError(ctx, "generate signed URL", "failed to generate signed URL", err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"url": url})
}
