package handler

import (
	"net/http"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/gin-gonic/gin"
)

// SignedURLHandler is an API handler that returns a signed URL.
func (h *Handler) SignedURLHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	arch := ctx.Param("arch")
	name := ctx.Query("name")

	if repoName == "" || arch == "" || name == "" {
		ctx.JSON(http.StatusBadRequest, domain.APIError{
			Message: "repository name, architecture, and file name are required",
		})
		return
	}

	url, err := h.s.SignedURL(repoName, arch, name)
	if url == "" && err == nil {
		ctx.JSON(http.StatusNoContent, domain.APIError{
			Message: "No signed URL available for the requested file",
		})
		return
	}
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, domain.APIError{
			Message: "failed to generate signed URL",
			Reason:  err,
		})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"url": url})
}
