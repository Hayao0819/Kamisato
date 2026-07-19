package handler

import (
	stderrors "errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"

	"github.com/Hayao0819/Kamisato/internal/limits"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/pkg/pacman/pkgfile"
)

func formFileWithValidate(ctx *gin.Context, name string, maxsize int) (*multipart.FileHeader, error) {
	pkgHeader, err := ctx.FormFile(name)
	if err != nil {
		return nil, fmt.Errorf("get form err: %w", err)
	}
	if pkgHeader.Size == 0 {
		return nil, fmt.Errorf("file is empty")
	}
	if limits.Exceeds(pkgHeader.Size, maxsize) {
		return nil, fmt.Errorf("file is too large")
	}
	return pkgHeader, nil
}

func (h *Handler) BlinkyUploadHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	if repoName == "" {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "repository name is required"})
		return
	}
	// Bound the body before spooling so an oversized upload is rejected as bytes
	// arrive, not after the whole body is already on disk/in memory.
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, limits.MultipartBytes(h.cfg.MaxSize))
	if err := ctx.Request.ParseMultipartForm(10 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if stderrors.As(err, &maxErr) {
			ctx.JSON(http.StatusRequestEntityTooLarge, domain.APIError{Message: "upload exceeds max_size"})
			return
		}
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("parse form err: %s", err.Error())})
		return
	}
	if ctx.Request.MultipartForm != nil {
		defer func() {
			if err := ctx.Request.MultipartForm.RemoveAll(); err != nil {
				slog.Warn("failed to remove multipart temporary files", "err", err)
			}
		}()
	}
	var names []string
	if ctx.Request.MultipartForm != nil {
		names = lo.Keys(ctx.Request.MultipartForm.File)
	}
	slog.Debug("BlinkyUploadHandler", "repo", repoName, "form names", names)
	if !lo.Contains(names, "package") {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "no package file found in the request"})
		return
	}
	if !lo.Contains(names, "signature") && h.cfg.RequireSign {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "signature file is required"})
		return
	}
	pkgHeader, err := formFileWithValidate(ctx, "package", h.cfg.MaxSize)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("get package form err: %s", err.Error())})
		return
	}
	sigHeader, err := ctx.FormFile("signature")
	if err != nil {
		if h.cfg.RequireSign {
			ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("get signature form err: %s", err.Error())})
			return
		} else {
			sigHeader = nil
			slog.Warn("failed to get signature form", "error", err.Error())
		}
	}
	if sigHeader != nil && (sigHeader.Size == 0 || sigHeader.Size > limits.MaxSignatureBytes) {
		ctx.JSON(http.StatusRequestEntityTooLarge, domain.APIError{Message: "signature is empty or too large"})
		return
	}
	pkgStream, err := formFileStream(pkgHeader)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("open file err: %s", err.Error())})
		return
	}
	defer pkgStream.Close()
	var sigStream *stream.FileStream
	if sigHeader != nil {
		sigStream, err = formFileStream(sigHeader)
		if err != nil {
			if h.cfg.RequireSign {
				ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("open signature file err: %s", err.Error())})
				return
			} else {
				sigStream = nil
				slog.Warn("failed to open signature file", "error", err.Error())
			}
		}
	}
	if sigStream != nil {
		defer sigStream.Close()
	}
	files := domain.UploadFiles{
		PkgFile: pkgStream,
	}
	if sigStream != nil {
		files.SigFile = sigStream
	}
	if err := h.s.UploadFile(repoName, &files); err != nil {
		ctx.JSON(errToStatus(err), domain.APIError{Message: fmt.Sprintf("upload file err: %s", err.Error())})
		return
	}
	ctx.String(http.StatusOK, fmt.Sprintf("'%s' uploaded!", pkgHeader.Filename))
}

// BatchUploadHandler publishes several packages atomically (one RepoAddBatch per arch),
// matching a "<name>.sig" signature to each package. RequireSign and signature
// verification are enforced by the service per package.
func (h *Handler) BatchUploadHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	if repoName == "" {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "repository name is required"})
		return
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, limits.BatchMultipartBytes(h.cfg.MaxBatchBytes, h.cfg.MaxSize))
	if err := ctx.Request.ParseMultipartForm(10 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if stderrors.As(err, &maxErr) {
			ctx.JSON(http.StatusRequestEntityTooLarge, domain.APIError{Message: "batch upload exceeds max_batch_bytes"})
			return
		}
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("parse form err: %s", err.Error())})
		return
	}
	form := ctx.Request.MultipartForm
	if form != nil {
		defer func() {
			if err := form.RemoveAll(); err != nil {
				slog.Warn("failed to remove multipart temporary files", "err", err)
			}
		}()
	}
	if form == nil || len(form.File["package"]) == 0 {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "no package files found in the request"})
		return
	}
	if len(form.File["package"]) > limits.BatchPackages(h.cfg.MaxBatchPackages) {
		ctx.JSON(http.StatusRequestEntityTooLarge, domain.APIError{Message: "too many packages in one batch"})
		return
	}
	if len(form.File["signature"]) > len(form.File["package"]) {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "batch contains more signatures than packages"})
		return
	}
	for field := range form.File {
		if field != "package" && field != "signature" {
			ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("unexpected multipart file field %q", field)})
			return
		}
	}
	if len(form.Value) != 0 {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "unexpected multipart value field"})
		return
	}

	// Match each signature to the package it signs by the "<pkg>.sig" convention.
	sigByArchive := make(map[string]*multipart.FileHeader, len(form.File["signature"]))
	var aggregate int64
	for _, sh := range form.File["signature"] {
		if sh.Size == 0 || sh.Size > limits.MaxSignatureBytes {
			ctx.JSON(http.StatusRequestEntityTooLarge, domain.APIError{Message: fmt.Sprintf("signature %q is empty or too large", sh.Filename)})
			return
		}
		artifact, err := pkgfile.Parse(sh.Filename)
		if err != nil || !artifact.IsSignature() {
			ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("signature %q must use the <package>.sig filename", sh.Filename)})
			return
		}
		if _, duplicate := sigByArchive[artifact.ArchiveFilename()]; duplicate {
			ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("duplicate signature %q", sh.Filename)})
			return
		}
		sigByArchive[artifact.ArchiveFilename()] = sh
		aggregate += sh.Size
	}
	packageNames := make(map[string]bool, len(form.File["package"]))
	for _, ph := range form.File["package"] {
		artifact, err := pkgfile.Parse(ph.Filename)
		if err != nil || artifact.IsSignature() {
			ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("package %q has an invalid package filename", ph.Filename)})
			return
		}
		if packageNames[ph.Filename] {
			ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("duplicate package %q", ph.Filename)})
			return
		}
		packageNames[ph.Filename] = true
		aggregate += ph.Size
	}
	for archive, signature := range sigByArchive {
		if !packageNames[archive] {
			ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("signature %q has no matching package", signature.Filename)})
			return
		}
	}
	if aggregate > limits.BatchBytes(h.cfg.MaxBatchBytes, h.cfg.MaxSize) {
		ctx.JSON(http.StatusRequestEntityTooLarge, domain.APIError{Message: "batch file data exceeds max_batch_bytes"})
		return
	}

	var files []*domain.UploadFiles
	var closers []io.Closer
	defer func() {
		for _, c := range closers {
			_ = c.Close()
		}
	}()

	for _, ph := range form.File["package"] {
		if ph.Size == 0 {
			ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("package %q is empty", ph.Filename)})
			return
		}
		if limits.Exceeds(ph.Size, h.cfg.MaxSize) {
			ctx.JSON(http.StatusRequestEntityTooLarge, domain.APIError{Message: fmt.Sprintf("package %q is too large", ph.Filename)})
			return
		}
		pkgStream, err := formFileStream(ph)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("open package %q err: %s", ph.Filename, err.Error())})
			return
		}
		closers = append(closers, pkgStream)
		uf := &domain.UploadFiles{PkgFile: pkgStream}
		if sh, ok := sigByArchive[ph.Filename]; ok {
			sigStream, err := formFileStream(sh)
			if err != nil {
				ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("open signature for %q err: %s", ph.Filename, err.Error())})
				return
			}
			closers = append(closers, sigStream)
			uf.SigFile = sigStream
		}
		files = append(files, uf)
	}

	if err := h.s.UploadFiles(repoName, files); err != nil {
		ctx.JSON(errToStatus(err), domain.APIError{Message: fmt.Sprintf("upload err: %s", err.Error())})
		return
	}
	ctx.String(http.StatusOK, fmt.Sprintf("%d package(s) uploaded!", len(files)))
}

// PresignUploadHandler is a compatibility tombstone for clients released while
// the unsafe final-key direct-upload protocol existed. It must remain a fixed
// 501 until a distinct, opaque staging-intent protocol replaces that design.
func (h *Handler) PresignUploadHandler(ctx *gin.Context) {
	ctx.JSON(http.StatusNotImplemented, domain.APIError{Message: "presigned upload is disabled until the staging-intent protocol is available"})
}

// FinalizeUploadHandler is the matching compatibility tombstone. There is no
// service or blob-store final-key upload capability behind this endpoint.
func (h *Handler) FinalizeUploadHandler(ctx *gin.Context) {
	ctx.JSON(http.StatusNotImplemented, domain.APIError{Message: "presigned upload is disabled until the staging-intent protocol is available"})
}
