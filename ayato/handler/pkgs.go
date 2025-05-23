package handler

import (
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
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, pkgs)
}
