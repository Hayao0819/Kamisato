package handler

import (
	"net/http"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/domain"
)

func (h *RepositoryHandler) AllPkgsHandler(ctx *gin.Context) {
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
	pkgs, err := h.reader.Pkgs(repoName, archName)
	if err != nil {
		respondServiceError(ctx, "get packages", "failed to get packages", err)
		return
	}
	ctx.JSON(http.StatusOK, pkgs)
}

func (h *RepositoryHandler) PkgDetailHandler(ctx *gin.Context) {
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
	pkgDetail, err := h.reader.PkgDetail(repoName, archName, pkgName)
	if err != nil {
		respondServiceError(ctx, "get package detail", "failed to get package detail", err)
		return
	}
	ctx.JSON(http.StatusOK, pkgDetail)
}

func (h *RepositoryHandler) PkgSignatureHandler(ctx *gin.Context) {
	repoName, archName, pkgName, ok := pkgRouteParams(ctx)
	if !ok {
		return
	}
	signature, err := h.reader.PkgSignature(repoName, archName, pkgName)
	if err != nil {
		respondServiceError(ctx, "get package signature", "failed to get package signature", err)
		return
	}
	ctx.JSON(http.StatusOK, signature)
}

func pkgRouteParams(ctx *gin.Context) (repo, arch, name string, ok bool) {
	repo = ctx.Param("repo")
	arch = ctx.Param("arch")
	name = ctx.Param("name")
	if repo == "" {
		respondError(ctx, http.StatusBadRequest, "repository name is required")
		return "", "", "", false
	}
	if arch == "" {
		respondError(ctx, http.StatusBadRequest, "architecture name is required")
		return "", "", "", false
	}
	if name == "" {
		respondError(ctx, http.StatusBadRequest, "package name is required")
		return "", "", "", false
	}
	return repo, arch, name, true
}

func (h *RepositoryHandler) PkgFilesHandler(ctx *gin.Context) {
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
	files, err := h.reader.PkgFiles(repoName, archName, pkgName)
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
