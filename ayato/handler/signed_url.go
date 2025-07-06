package handler

import (
	"net/http"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/gin-gonic/gin"
)

func (h *Handler) SignedURLHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	arch := ctx.Param("arch")
	name := ctx.Query("name")

	if repoName == "" || arch == "" || name == "" {
		// ctx.String(http.StatusBadRequest, "repository name, architecture, and file name are required")
		ctx.JSON(http.StatusBadRequest, domain.APIError{
			Message: "repository name, architecture, and file name are required",
		})
		return
	}

	url, err := h.s.SignedURL(repoName, arch, name)
	if url == "" && err == nil {
		// ctx.String(http.StatusOK, )
		ctx.JSON(http.StatusNoContent, domain.APIError{
			Message: "No signed URL available for the requested file",
		})
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
