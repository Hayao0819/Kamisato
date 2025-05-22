package handler

import (
	"fmt"
	"net/http"
	"os"
	"path"

	"github.com/gin-gonic/gin"
)

func (h *Handler) BlinkyUploadHandler(ctx *gin.Context) {
	// Check if the request contains a file
	repoName := ctx.Param("repo")
	// Validate the repository
	if repoName == "" {
		ctx.String(http.StatusBadRequest, "repository name is required")
		return
	}

	// TODO: Check signature file

	// Validate the file
	pkgHeader, err := ctx.FormFile("package")
	if err != nil {
		ctx.String(http.StatusBadRequest, fmt.Sprintf("get form err: %s", err.Error()))
		return
	}
	if pkgHeader.Size == 0 {
		ctx.String(http.StatusBadRequest, "file is empty")
		return
	}
	if pkgHeader.Size > int64(h.cfg.MaxSize) {
		ctx.String(http.StatusBadRequest, "file is too large")
		return
	}

	// Create a temporary directory to save the file
	tmpDir, err := os.MkdirTemp("", "ayato-upload-*")
	if err != nil {
		ctx.String(http.StatusInternalServerError, fmt.Sprintf("create temp dir err: %s", err.Error()))
		return
	}
	defer os.RemoveAll(tmpDir)

	// Save the file to the temporary directory
	if err := ctx.SaveUploadedFile(pkgHeader, path.Join(tmpDir, pkgHeader.Filename)); err != nil {
		ctx.String(http.StatusInternalServerError, fmt.Sprintf("upload file err: %s", err.Error()))
		return
	}

	// Upload the file to the repository
	pkgfile := path.Join(tmpDir, pkgHeader.Filename)
	if err := h.s.UploadPkgFile(repoName, [2]string{pkgfile, ""}); err != nil {
		ctx.String(http.StatusInternalServerError, fmt.Sprintf("upload file err: %s", err.Error()))
		return
	}

	ctx.String(http.StatusOK, fmt.Sprintf("'%s' uploaded!", pkgHeader.Filename))
}
