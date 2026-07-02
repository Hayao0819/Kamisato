package handler

import (
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

// defaultMaxUploadBytes caps an upload body when cfg.MaxSize is unset, so a
// single authenticated request cannot spool an unbounded body into memory or the
// tmpfs-backed /tmp on Cloud Run.
const defaultMaxUploadBytes = 512 << 20

// maxUploadBytes is the byte ceiling enforced before the multipart body is
// spooled. cfg.MaxSize bounds a single package; the margin covers multipart
// framing and the small detached-signature part.
func maxUploadBytes(maxSize int) int64 {
	if maxSize > 0 {
		return int64(maxSize) + (1 << 20)
	}
	return defaultMaxUploadBytes
}

func formFileWithValidate(ctx *gin.Context, name string, maxsize int) (*multipart.FileHeader, error) {
	pkgHeader, err := ctx.FormFile(name)
	if err != nil {
		return nil, fmt.Errorf("get form err: %w", err)
	}
	if pkgHeader.Size == 0 {
		return nil, fmt.Errorf("file is empty")
	}
	if maxsize > 0 && pkgHeader.Size > int64(maxsize) {
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
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxUploadBytes(h.cfg.MaxSize))
	if err := ctx.Request.ParseMultipartForm(10 << 20); err != nil {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("parse form err: %s", err.Error())})
		return
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

// BatchUploadHandler publishes several packages atomically. The multipart form
// carries one or more "package" files and any matching "signature" files (a
// signature for "<name>" is named "<name>.sig"); ayato registers them all with
// one RepoAddBatch per arch. Use it instead of the single PUT to publish a split
// package or a rebuild set as one atomic database update. RequireSign and
// signature verification are enforced by the service per package.
func (h *Handler) BatchUploadHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	if repoName == "" {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "repository name is required"})
		return
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxUploadBytes(h.cfg.MaxSize))
	if err := ctx.Request.ParseMultipartForm(10 << 20); err != nil {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("parse form err: %s", err.Error())})
		return
	}
	form := ctx.Request.MultipartForm
	if form == nil || len(form.File["package"]) == 0 {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "no package files found in the request"})
		return
	}

	// Match each signature to the package it signs by the "<pkg>.sig" convention.
	sigByName := make(map[string]*multipart.FileHeader, len(form.File["signature"]))
	for _, sh := range form.File["signature"] {
		sigByName[sh.Filename] = sh
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
		if h.cfg.MaxSize > 0 && ph.Size > int64(h.cfg.MaxSize) {
			ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("package %q is too large", ph.Filename)})
			return
		}
		pkgStream, err := formFileStream(ph)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, domain.APIError{Message: fmt.Sprintf("open package %q err: %s", ph.Filename, err.Error())})
			return
		}
		closers = append(closers, pkgStream)
		uf := &domain.UploadFiles{PkgFile: pkgStream}
		if sh, ok := sigByName[ph.Filename+".sig"]; ok {
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
