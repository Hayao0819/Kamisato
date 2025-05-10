package handler

import (
	"fmt"
	"net/http"
	"os"
	"path"

	"github.com/gin-gonic/gin"
)

func (h *Handler) UploadHandler(ctx *gin.Context) {
	// Check if the request contains a file
	repoName := ctx.Param("repo")
	file, err := ctx.FormFile("package")
	if err != nil {
		ctx.String(http.StatusBadRequest, fmt.Sprintf("get form err: %s", err.Error()))
		return
	}

	// TODO: Check signature file

	// Validate the file
	if file.Size == 0 {
		ctx.String(http.StatusBadRequest, "file is empty")
		return
	}
	fmt.Println(file.Size, h.cfg.MaxSize)
	if file.Size > int64(h.cfg.MaxSize) {
		ctx.String(http.StatusBadRequest, "file is too large")
		return
	}

	// Validate the repository
	if repoName == "" {
		ctx.String(http.StatusBadRequest, "repository name is required")
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
	if err := ctx.SaveUploadedFile(file, path.Join(tmpDir, file.Filename)); err != nil {
		ctx.String(http.StatusInternalServerError, fmt.Sprintf("upload file err: %s", err.Error()))
		return
	}

	// fullPkgBinary := path.Join(repoDbPath, file.Filename)

	// Upload the file to the repository
	pkgfile := path.Join(tmpDir, file.Filename)
	if err := h.s.UploadPkgFile(repoName, [2]string{pkgfile, ""}); err != nil {
		ctx.String(http.StatusInternalServerError, fmt.Sprintf("upload file err: %s", err.Error()))
		return
	}

	ctx.String(http.StatusOK, fmt.Sprintf("'%s' uploaded!", file.Filename))
}
