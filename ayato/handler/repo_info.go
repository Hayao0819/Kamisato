package handler

import (
	"log/slog"
	"net/http"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/gin-gonic/gin"
)

// ReposHandler handles requests to get a list of all repositories.
func (h *Handler) ReposHandler(ctx *gin.Context) {
	repoNames, err := h.s.RepoNames() // Assuming a service method exists
	if err != nil {
		slog.Error("Failed to get repository names", "error", err)
		ctx.JSON(http.StatusInternalServerError, domain.APIError{Message: err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, repoNames)
}

// ArchesHandler handles requests to get a list of architectures for a given repository.
func (h *Handler) ArchesHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	if repoName == "" {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "Repository name is required"})
		return
	}

	archNames, err := h.s.Arches(repoName) // Assuming a service method exists
	if err != nil {
		slog.Error("Failed to get architectures for repository", "repo", repoName, "error", err)
		ctx.JSON(http.StatusInternalServerError, domain.APIError{Message: err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, archNames)
}
