package handler

import (
	stderrors "errors"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/platform"
	"github.com/Hayao0819/Kamisato/internal/limits"
)

type uploadFileError struct {
	status int
	reason string
}

func (e *uploadFileError) Error() string { return e.reason }

func validateUploadFile(header *multipart.FileHeader, maxSize int) error {
	if header.Size == 0 {
		return &uploadFileError{status: http.StatusBadRequest, reason: "is empty"}
	}
	if limits.Exceeds(header.Size, maxSize) {
		return &uploadFileError{status: http.StatusRequestEntityTooLarge, reason: "is too large"}
	}
	return nil
}

func uploadFileErrorStatus(err error) int {
	var validationErr *uploadFileError
	if stderrors.As(err, &validationErr) {
		return validationErr.status
	}
	return http.StatusBadRequest
}

func validSignatureSize(header *multipart.FileHeader) bool {
	return header.Size > 0 && header.Size <= limits.MaxSignatureBytes
}

func formFileWithValidate(ctx *gin.Context, name string, maxSize int) (*multipart.FileHeader, error) {
	header, err := ctx.FormFile(name)
	if err != nil {
		return nil, fmt.Errorf("get form err: %w", err)
	}
	if err := validateUploadFile(header, maxSize); err != nil {
		return nil, err
	}
	return header, nil
}

func parseUploadForm(
	ctx *gin.Context,
	maxBytes int64,
	limitMessage string,
) (*multipart.Form, bool) {
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxBytes)
	if err := ctx.Request.ParseMultipartForm(10 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if stderrors.As(err, &maxErr) {
			respondError(ctx, http.StatusRequestEntityTooLarge, limitMessage)
		} else {
			respondError(ctx, http.StatusBadRequest, "invalid multipart form")
		}
		return nil, false
	}
	return ctx.Request.MultipartForm, true
}

func removeUploadForm(form *multipart.Form) {
	if form == nil {
		return
	}
	if err := form.RemoveAll(); err != nil {
		slog.Warn("failed to remove multipart temporary files", "err", err)
	}
}

func (h *PublicationHandler) BlinkyUploadHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	if repoName == "" {
		respondError(ctx, http.StatusBadRequest, "repository name is required")
		return
	}
	form, ok := parseUploadForm(
		ctx,
		limits.MultipartBytes(h.cfg.MaxSize),
		"upload exceeds max_size",
	)
	if !ok {
		return
	}
	defer removeUploadForm(form)

	var names []string
	if form != nil {
		names = lo.Keys(form.File)
	}
	slog.Debug("BlinkyUploadHandler", "repo", repoName, "form names", names)
	if !lo.Contains(names, "package") {
		respondError(ctx, http.StatusBadRequest, "no package file found in the request")
		return
	}
	if !lo.Contains(names, "signature") && h.cfg.RequireSign {
		respondError(ctx, http.StatusBadRequest, "signature file is required")
		return
	}

	pkgHeader, err := formFileWithValidate(ctx, "package", h.cfg.MaxSize)
	if err != nil {
		respondError(ctx, uploadFileErrorStatus(err), "invalid package file")
		return
	}
	sigHeader, err := ctx.FormFile("signature")
	if err != nil {
		if h.cfg.RequireSign {
			respondError(ctx, http.StatusBadRequest, "invalid signature file")
			return
		}
		sigHeader = nil
		slog.Warn("failed to get signature form", "error", err.Error())
	}
	if sigHeader != nil && !validSignatureSize(sigHeader) {
		respondError(ctx, http.StatusRequestEntityTooLarge, "signature is empty or too large")
		return
	}

	pkgStream, err := formFileStream(pkgHeader)
	if err != nil {
		respondLoggedError(ctx, http.StatusInternalServerError, "open uploaded package", "failed to read uploaded package", err)
		return
	}
	defer func() { _ = pkgStream.Close() }()
	var sigStream *platform.FileStream
	if sigHeader != nil {
		sigStream, err = formFileStream(sigHeader)
		if err != nil {
			if h.cfg.RequireSign {
				respondLoggedError(ctx, http.StatusInternalServerError, "open uploaded signature", "failed to read uploaded signature", err)
				return
			}
			sigStream = nil
			slog.Warn("failed to open signature file", "error", err.Error())
		}
	}
	if sigStream != nil {
		defer func() { _ = sigStream.Close() }()
	}

	files := domain.UploadFiles{PkgFile: pkgStream}
	if sigStream != nil {
		files.SigFile = sigStream
	}
	if err := h.uploader.UploadFile(repoName, &files); err != nil {
		respondServiceError(ctx, "upload package", "failed to upload package", err)
		return
	}
	ctx.String(http.StatusOK, fmt.Sprintf("'%s' uploaded!", pkgHeader.Filename))
}
