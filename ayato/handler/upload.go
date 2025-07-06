package handler

import (
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	utils "github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

func formFileWithValidate(ctx *gin.Context, name string, maxsize int) (*multipart.FileHeader, error) {
	pkgHeader, err := ctx.FormFile(name)
	if err != nil {
		// ctx.String(http.StatusBadRequest, fmt.Sprintf("get form err: %s", err.Error()))
		return nil, fmt.Errorf("get form err: %w", err)
	}
	if pkgHeader.Size == 0 {
		// ctx.String(http.StatusBadRequest, "file is empty")
		return nil, fmt.Errorf("file is empty")
	}
	if maxsize > 0 && pkgHeader.Size > int64(maxsize) {
		// ctx.String(http.StatusBadRequest, "file is too large")
		return nil, fmt.Errorf("file is too large")
	}
	return pkgHeader, nil

}

func (h *Handler) BlinkyUploadHandler(ctx *gin.Context) {
	// Check if the request contains a file
	repoName := ctx.Param("repo")
	// Validate the repository
	if repoName == "" {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "repository name is required"})
		return
	}

	// ここの数値はいい感じに調整する
	if err := ctx.Request.ParseMultipartForm(10 << 20); err != nil {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("parse form err: %s", err.Error())})
		return
	}

	// Check multipart form
	names := utils.MultipartFormNames(ctx.Request)
	slog.Debug("BlinkyUploadHandler", "repo", repoName, "form names", names)
	if !lo.Contains(names, "package") {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "no package file found in the request"})
		return
	}
	if !lo.Contains(names, "signature") && h.cfg.RequireSign {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "signature file is required"})
		return
	}

	// Validate the file
	pkgHeader, err := formFileWithValidate(ctx, "package", h.cfg.MaxSize)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("get package form err: %s", err.Error())})
		return
	}

	// Validate the signature file
	sigHeader, err := ctx.FormFile("signature")
	if err != nil {
		if h.cfg.RequireSign {
			ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("get signature form err: %s", err.Error())})
			return
		} else {
			sigHeader = nil // Signature is optional if not required
			slog.Warn("failed to get signature form", "error", err.Error())
		}
	}

	pkgStream, err := formFileStream(pkgHeader)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("open file err: %s", err.Error())})
		return
	}

	var sigStream *stream.FileStream
	if sigHeader != nil {
		sigStream, err = formFileStream(sigHeader)
		if err != nil {
			if h.cfg.RequireSign {
				ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("open signature file err: %s", err.Error())})
				return
			} else {
				sigStream = nil // Signature is optional if not required
				slog.Warn("failed to open signature file", "error", err.Error())
			}
		}
	}

	// Upload the file to the repository
	files := domain.UploadFiles{
		PkgFile: pkgStream,
	}
	if sigStream != nil {
		files.SigFile = sigStream
	}
	if err := h.s.UploadFile(repoName, &files); err != nil {
		ctx.JSON(http.StatusInternalServerError, domain.APIError{Message: fmt.Sprintf("upload file err: %s", err.Error())})
		return
	}

	ctx.String(http.StatusOK, fmt.Sprintf("'%s' uploaded!", pkgHeader.Filename))
}
