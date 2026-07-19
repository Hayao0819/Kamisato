package handler

import (
	"net/http"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/domain"
)

func (h *Handler) AllPkgsHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	archName := ctx.Param("arch")
	if repoName == "" {
		respondError(ctx, http.StatusBadRequest, "repository name is required")
		return
	}
	if archName == "" {
		respondError(ctx, http.StatusBadRequest, "architecture name is required")
		return
	}
	pkgs, err := h.s.Pkgs(repoName, archName)
	if err != nil {
		respondServiceError(ctx, "get packages", "failed to get packages", err)
		return
	}
	ctx.JSON(http.StatusOK, pkgs)
}

func (h *Handler) PkgDetailHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	archName := ctx.Param("arch")
	pkgName := ctx.Param("name")
	if repoName == "" {
		respondError(ctx, http.StatusBadRequest, "repository name is required")
		return
	}
	if archName == "" {
		respondError(ctx, http.StatusBadRequest, "architecture name is required")
		return
	}
	if pkgName == "" {
		respondError(ctx, http.StatusBadRequest, "package name is required")
		return
	}
	pkgDetail, err := h.s.PkgDetail(repoName, archName, pkgName)
	if err != nil {
		respondServiceError(ctx, "get package detail", "failed to get package detail", err)
		return
	}
	ctx.JSON(http.StatusOK, pkgDetail)
}

func (h *Handler) PkgFilesHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	archName := ctx.Param("arch")
	pkgName := ctx.Param("name")
	if repoName == "" {
		respondError(ctx, http.StatusBadRequest, "repository name is required")
		return
	}
	if archName == "" {
		respondError(ctx, http.StatusBadRequest, "architecture name is required")
		return
	}
	if pkgName == "" {
		respondError(ctx, http.StatusBadRequest, "package name is required")
		return
	}
	files, err := h.s.PkgFiles(repoName, archName, pkgName)
	if errors.Is(err, domain.ErrNotImplemented) {
		respondError(ctx, http.StatusNotImplemented, "package file listing is not implemented")
		return
	}
	if err != nil {
		respondServiceError(ctx, "get package files", "failed to get package files", err)
		return
	}
	ctx.JSON(http.StatusOK, files)
}
