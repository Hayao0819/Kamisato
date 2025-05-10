package handler

import (
	"fmt"
	"net/http"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/repo"
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
	if err := repo.ValidatePkgHeader(file); err != nil {
		ctx.String(http.StatusBadRequest, fmt.Sprintf("validate file err: %s", err.Error()))
		return
	}

	tmpDir, err := os.MkdirTemp("", "ayato-upload-*")
	if err != nil {
		ctx.String(http.StatusInternalServerError, fmt.Sprintf("create temp dir err: %s", err.Error()))
		return
	}
	defer os.RemoveAll(tmpDir)

	// Save the file to the repository
	if err := ctx.SaveUploadedFile(file, path.Join(tmpDir, file.Filename)); err != nil {
		ctx.String(http.StatusInternalServerError, fmt.Sprintf("upload file err: %s", err.Error()))
		return
	}

	// fullPkgBinary := path.Join(repoDbPath, file.Filename)

	pkgfile := path.Join(tmpDir, file.Filename)
	if err := h.s.UploadPkgFile(repoName, [2]string{pkgfile, ""}); err != nil {
		ctx.String(http.StatusInternalServerError, fmt.Sprintf("upload file err: %s", err.Error()))
		return
	}

	ctx.String(http.StatusOK, fmt.Sprintf("'%s' uploaded!", file.Filename))
}
