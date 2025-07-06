package handler

import (
	"log/slog"
	"net/http"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/gin-gonic/gin"
)

func (h *Handler) AllPkgsHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	archName := ctx.Param("arch")
	if repoName == "" {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "Repository name is required"})
		return
	}
	if archName == "" {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "Arch name is required"})
		return
	}

	pkgs, err := h.s.Pkgs(repoName, archName)
	if err != nil {
		slog.Error("Failed to get packages", "error", err)
		ctx.JSON(http.StatusInternalServerError, domain.APIError{
			Message: "failed to get packages",
			Reason:  err,
		})
		return
	}

	ctx.JSON(http.StatusOK, pkgs)

}

func (h *Handler) PkgDetailHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	archName := ctx.Param("arch")
	pkgName := ctx.Param("name")
	if repoName == "" {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "Repository name is required"})
		return
	}
	if archName == "" {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "Arch name is required"})
		return
	}
	if pkgName == "" {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "Package name is required"})
		return
	}

	pkgDetail, err := h.s.PkgDetail(repoName, archName, pkgName)
	if err != nil {
		slog.Error("Failed to get package detail", "error", err)
		ctx.JSON(http.StatusInternalServerError, domain.APIError{Message: err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, pkgDetail)
}

func (h *Handler) PkgFilesHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	archName := ctx.Param("arch")
	pkgName := ctx.Param("name")
	if repoName == "" {
		// ctx.String(http.StatusBadRequest, "Repository name is required")
		// ctx.JSON(http.StatusBadRequest, gin.H{"error": "Repository name is required"})
		ctx.JSON(http.StatusBadRequest, domain.APIError{
			Message: "Repository name is required",
		})
		return
	}
	if archName == "" {
		// ctx.String(http.StatusBadRequest, "Arch name is required")
		// ctx.JSON(http.StatusBadRequest, gin.H{"error": "Arch name is required"})
		ctx.JSON(http.StatusBadRequest, domain.APIError{
			Message: "Arch name is required",
		})
		return
	}
	if pkgName == "" {
		// ctx.String(http.StatusBadRequest, "Package name is required")
		// ctx.JSON(http.StatusBadRequest, gin.H{"error": "Package name is required"})
		ctx.JSON(http.StatusBadRequest, domain.APIError{
			Message: "Package name is required",
		})
		return
	}

	files, err := h.s.PkgFiles(repoName, archName, pkgName)
	if err != nil {
		slog.Error("Failed to get package files", "error", err)
		// ctx.String(http.StatusInternalServerError, err.Error())
		ctx.JSON(http.StatusInternalServerError, domain.APIError{
			Message: "failed to get package files",
			Reason:  err,
		})
		return
	}

	ctx.JSON(http.StatusOK, files)
}

func (h *Handler) PkgDetailFile(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	archName := ctx.Param("arch")
	pkgName := ctx.Param("name")
	if repoName == "" {
		ctx.String(http.StatusBadRequest, "Repository name is required")
		return
	}
	if archName == "" {
		ctx.String(http.StatusBadRequest, "Arch name is required")
		return
	}
	if pkgName == "" {
		ctx.String(http.StatusBadRequest, "Package name is required")
		return
	}
	pkgDetail, err := h.s.PkgDetail(repoName, archName, pkgName)
	if err != nil {
		slog.Error("Failed to get package detail", "error", err)
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}
	ctx.JSON(http.StatusOK, pkgDetail)
}
