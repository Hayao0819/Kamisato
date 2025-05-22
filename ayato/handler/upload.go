package handler

import (
	"fmt"
	"net/http"

	"github.com/Hayao0819/Kamisato/ayato/domain"
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

	pkgStream, err := formFileStream(pkgHeader)
	if err != nil {
		ctx.String(http.StatusBadRequest, fmt.Sprintf("open file err: %s", err.Error()))
		return
	}

	// Upload the file to the repository
	// pkgfile := path.Join(tmpDir, pkgHeader.Filename)
	if err := h.s.UploadPkgFile(repoName, &domain.UploadFiles{
		PkgFile: pkgStream,
	}); err != nil {
		ctx.String(http.StatusInternalServerError, fmt.Sprintf("upload file err: %s", err.Error()))
		return
	}

	ctx.String(http.StatusOK, fmt.Sprintf("'%s' uploaded!", pkgHeader.Filename))
}
