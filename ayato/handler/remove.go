package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) RemoveHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	packageName := ctx.Param("name")
	if packageName == "" {
		ctx.String(http.StatusBadRequest, "Package name is required")
		return
	}
	if repoName == "" {
		ctx.String(http.StatusBadRequest, "Repository name is required")
		return
	}

	if err := h.s.RemovePkgFile(repoName, packageName); err != nil {
		ctx.String(http.StatusInternalServerError, fmt.Sprintf("Remove package file err: %s", err.Error()))
		return
	}

	// Remove the package from the repository
	// useSignedDB := false
	// var gnupgDir *string
	// err := utils.RepoRemove(cfg.DBPath, packageName, useSignedDB, gnupgDir)
	// if err != nil {
	// 	ctx.String(http.StatusInternalServerError, fmt.Sprintf("repo-remove err: %s", err.Error()))
	// 	return
	// }

	ctx.String(http.StatusOK, fmt.Sprintf("'%s' removed!", packageName))
}
