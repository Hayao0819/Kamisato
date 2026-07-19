package handler

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/internal/limits"
	pacmanpkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
)

// BatchUploadHandler publishes several packages atomically (one RepoAddBatch per
// arch), matching a "<name>.sig" signature to each package. RequireSign and
// signature verification are enforced by the service per package.
func (h *PublicationHandler) BatchUploadHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	if repoName == "" {
		respondError(ctx, http.StatusBadRequest, "repository name is required")
		return
	}
	form, ok := parseUploadForm(
		ctx,
		limits.BatchMultipartBytes(h.cfg.MaxBatchBytes, h.cfg.MaxSize),
		"batch upload exceeds max_batch_bytes",
	)
	if !ok {
		return
	}
	defer removeUploadForm(form)
	if form == nil || len(form.File["package"]) == 0 {
		respondError(ctx, http.StatusBadRequest, "no package files found in the request")
		return
	}
	if len(form.File["package"]) > limits.BatchPackages(h.cfg.MaxBatchPackages) {
		respondError(ctx, http.StatusRequestEntityTooLarge, "too many packages in one batch")
		return
	}
	if len(form.File["signature"]) > len(form.File["package"]) {
		respondError(ctx, http.StatusBadRequest, "batch contains more signatures than packages")
		return
	}
	for field := range form.File {
		if field != "package" && field != "signature" {
			respondError(ctx, http.StatusBadRequest, fmt.Sprintf("unexpected multipart file field %q", field))
			return
		}
	}
	if len(form.Value) != 0 {
		respondError(ctx, http.StatusBadRequest, "unexpected multipart value field")
		return
	}

	sigByArchive := make(map[string]*multipart.FileHeader, len(form.File["signature"]))
	var aggregate int64
	for _, signature := range form.File["signature"] {
		if !validSignatureSize(signature) {
			respondError(ctx, http.StatusRequestEntityTooLarge, fmt.Sprintf(
				"signature %q is empty or too large",
				signature.Filename,
			))
			return
		}
		artifact, err := pacmanpkg.ParseArtifact(signature.Filename)
		if err != nil || !artifact.IsSignature() {
			respondError(ctx, http.StatusBadRequest, fmt.Sprintf(
				"signature %q must use the <package>.sig filename",
				signature.Filename,
			))
			return
		}
		archive := artifact.ArchiveFilename()
		if _, duplicate := sigByArchive[archive]; duplicate {
			respondError(ctx, http.StatusBadRequest, fmt.Sprintf(
				"duplicate signature %q",
				signature.Filename,
			))
			return
		}
		sigByArchive[archive] = signature
		aggregate += signature.Size
	}

	packageNames := make(map[string]bool, len(form.File["package"]))
	for _, archive := range form.File["package"] {
		artifact, err := pacmanpkg.ParseArtifact(archive.Filename)
		if err != nil || artifact.IsSignature() {
			respondError(ctx, http.StatusBadRequest, fmt.Sprintf(
				"package %q has an invalid package filename",
				archive.Filename,
			))
			return
		}
		if packageNames[archive.Filename] {
			respondError(ctx, http.StatusBadRequest, fmt.Sprintf(
				"duplicate package %q",
				archive.Filename,
			))
			return
		}
		packageNames[archive.Filename] = true
		aggregate += archive.Size
	}
	for archive, signature := range sigByArchive {
		if !packageNames[archive] {
			respondError(ctx, http.StatusBadRequest, fmt.Sprintf(
				"signature %q has no matching package",
				signature.Filename,
			))
			return
		}
	}
	if aggregate > limits.BatchBytes(h.cfg.MaxBatchBytes, h.cfg.MaxSize) {
		respondError(ctx, http.StatusRequestEntityTooLarge, "batch file data exceeds max_batch_bytes")
		return
	}

	files, closers, ok := openBatchFiles(ctx, form.File["package"], sigByArchive, h.cfg.MaxSize)
	defer closeAll(closers)
	if !ok {
		return
	}
	if err := h.uploader.UploadFiles(repoName, files); err != nil {
		respondServiceError(ctx, "upload package batch", "failed to upload package batch", err)
		return
	}
	ctx.String(http.StatusOK, fmt.Sprintf("%d package(s) uploaded!", len(files)))
}

func openBatchFiles(
	ctx *gin.Context,
	archives []*multipart.FileHeader,
	signatures map[string]*multipart.FileHeader,
	maxSize int,
) ([]*domain.UploadFiles, []io.Closer, bool) {
	files := make([]*domain.UploadFiles, 0, len(archives))
	closers := make([]io.Closer, 0, len(archives)*2)
	for _, archive := range archives {
		if err := validateUploadFile(archive, maxSize); err != nil {
			respondError(ctx, uploadFileErrorStatus(err), fmt.Sprintf(
				"package %q %s",
				archive.Filename,
				err,
			))
			return nil, closers, false
		}
		pkgStream, err := formFileStream(archive)
		if err != nil {
			respondLoggedError(ctx, http.StatusInternalServerError, "open batch package", "failed to read uploaded package", err)
			return nil, closers, false
		}
		closers = append(closers, pkgStream)
		upload := &domain.UploadFiles{PkgFile: pkgStream}
		if signature, exists := signatures[archive.Filename]; exists {
			sigStream, err := formFileStream(signature)
			if err != nil {
				respondLoggedError(ctx, http.StatusInternalServerError, "open batch signature", "failed to read uploaded signature", err)
				return nil, closers, false
			}
			closers = append(closers, sigStream)
			upload.SigFile = sigStream
		}
		files = append(files, upload)
	}
	return files, closers, true
}

func closeAll(closers []io.Closer) {
	for _, closer := range closers {
		_ = closer.Close()
	}
}

// StagedUploadUnavailableHandler is the shared compatibility tombstone for
// presign and finalize clients released while the unsafe final-key direct-upload
// protocol existed.
func (h *PublicationHandler) StagedUploadUnavailableHandler(ctx *gin.Context) {
	respondError(ctx, http.StatusNotImplemented, "presigned upload is disabled until the staging-intent protocol is available")
}
