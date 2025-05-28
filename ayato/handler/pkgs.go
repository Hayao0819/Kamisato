package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) AllPkgsHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	archName := ctx.Param("arch")
	if repoName == "" {
		ctx.String(http.StatusBadRequest, "Repository name is required")
		return
	}
	if archName == "" {
		ctx.String(http.StatusBadRequest, "Arch name is required")
		return
	}

	pkgs, err := h.s.PacmanRepoPkgs(repoName, archName)
	if err != nil {
		slog.Error("Failed to get packages", "error", err)
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, pkgs)

}

func (h *Handler) PkgFilesHandler(ctx *gin.Context) {
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

	files, err := h.s.PacmanRepoPkgFiles(repoName, archName, pkgName)
	if err != nil {
		slog.Error("Failed to get package files", "error", err)
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, files)
}
