package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/domain"
)

type presignFileRequest struct {
	Name string `json:"name"`
	Size int64  `json:"size,omitempty"`
}

type presignRequest struct {
	Files []presignFileRequest `json:"files"`
}

type presignResponse struct {
	ID         string            `json:"id"`
	TTLSeconds int               `json:"ttl_seconds"`
	URLs       map[string]string `json:"urls"`
}

type commitFileEntry struct {
	Package   string `json:"package"`
	Signature string `json:"signature,omitempty"`
}

type commitRequest struct {
	ID    string            `json:"id"`
	Files []commitFileEntry `json:"files"`
}

// PresignUploadHandler grants presigned staging PUTs for a package upload,
// letting the bytes go client -> object storage directly instead of through
// the Cloudflare zone proxy's request-body cap. CommitUploadHandler then
// validates and publishes from storage.
func (h *PublicationHandler) PresignUploadHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	if repoName == "" {
		respondError(ctx, http.StatusBadRequest, "repository name is required")
		return
	}
	var req presignRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respondError(ctx, http.StatusBadRequest, "invalid request body")
		return
	}
	files := make([]domain.StagedFileRequest, 0, len(req.Files))
	for _, file := range req.Files {
		files = append(files, domain.StagedFileRequest{Name: file.Name, Size: file.Size})
	}
	grant, err := h.uploader.PresignUpload(repoName, files)
	if err != nil {
		h.respondStagedUploadError(ctx, "presign package upload", err)
		return
	}
	ctx.JSON(http.StatusOK, presignResponse{ID: grant.ID, TTLSeconds: grant.TTLSeconds, URLs: grant.URLs})
}

// CommitUploadHandler validates and publishes every file of a staged intent
// through the same pipeline as the multipart upload endpoint.
func (h *PublicationHandler) CommitUploadHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	if repoName == "" {
		respondError(ctx, http.StatusBadRequest, "repository name is required")
		return
	}
	var req commitRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respondError(ctx, http.StatusBadRequest, "invalid request body")
		return
	}
	entries := make([]domain.StagedCommitEntry, 0, len(req.Files))
	for _, file := range req.Files {
		entries = append(entries, domain.StagedCommitEntry{Package: file.Package, Signature: file.Signature})
	}
	if err := h.uploader.CommitUpload(repoName, req.ID, entries); err != nil {
		h.respondStagedUploadError(ctx, "commit staged upload", err)
		return
	}
	ctx.String(http.StatusOK, fmt.Sprintf("%d package(s) committed!", len(entries)))
}

// respondStagedUploadError keeps the tombstone response byte-for-byte when the
// blob backend has no staging capability, and the normal error mapping otherwise.
func (h *PublicationHandler) respondStagedUploadError(ctx *gin.Context, operation string, err error) {
	if errors.Is(err, domain.ErrNotImplemented) {
		h.StagedUploadUnavailableHandler(ctx)
		return
	}
	respondServiceError(ctx, operation, "failed to "+operation, err)
}
