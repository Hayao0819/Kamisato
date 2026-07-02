package handler

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/gin-gonic/gin"
)

func (h *Handler) ReposHandler(ctx *gin.Context) {
	repoNames, err := h.s.RepoNames()
	if err != nil {
		slog.Error("Failed to get repository names", "error", err)
		ctx.JSON(http.StatusInternalServerError, domain.APIError{Message: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, repoNames)
}

func (h *Handler) ArchesHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	if repoName == "" {
		ctx.JSON(http.StatusBadRequest, domain.APIError{Message: "Repository name is required"})
		return
	}
	archNames, err := h.s.Arches(repoName)
	if err != nil {
		slog.Error("Failed to get architectures for repository", "repo", repoName, "error", err)
		ctx.JSON(http.StatusInternalServerError, domain.APIError{Message: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, archNames)
}

func (h *Handler) RepoFileHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	arch := ctx.Param("arch")
	fileName := ctx.Param("file")

	slog.Debug("repoHandler", "repoName", repoName)

	// Offload egress to the object store when it can presign: redirect the client
	// straight to a presigned GET so the bytes never transit ayato (Cloud Run bills
	// egress, and packages are the bulk of it). pacman follows redirects. A backend
	// that cannot presign (localfs) returns "", so we fall through to streaming.
	if h.cfg == nil || h.cfg.RedirectDownloadsEnabled() {
		if url, err := h.s.SignedURL(repoName, arch, fileName); err == nil && url != "" {
			ctx.Redirect(http.StatusFound, url)
			return
		}
	}

	s, meta, err := h.s.GetFileWithMeta(repoName, arch, fileName)
	if err != nil {
		ctx.JSON(errToStatus(err), domain.APIError{
			Message: "failed to serve " + fileName,
			Reason:  err.Error(),
		})
		return
	}
	defer s.Close()

	// Advertise validators so a client can skip re-downloading an unchanged file:
	// pacman drives this off Last-Modified/If-Modified-Since (from the local .db
	// mtime), while HTTP caches and proxies use the ETag/If-None-Match.
	if meta.ETag != "" {
		ctx.Header("ETag", meta.ETag)
	}
	if !meta.LastModified.IsZero() {
		ctx.Header("Last-Modified", meta.LastModified.UTC().Format(http.TimeFormat))
	}
	if meta.ETag != "" || !meta.LastModified.IsZero() {
		ctx.Header("Cache-Control", "no-cache")
	}
	if notModified(ctx.Request, meta) {
		ctx.Status(http.StatusNotModified)
		return
	}

	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", s.FileName()))
	ctx.Header("Content-Type", s.ContentType())

	_, err = io.Copy(ctx.Writer, s)
	if err != nil {
		slog.Error("failed to write file to response", "error", err)
	}
}

// notModified evaluates the request's conditional headers against the served
// file's validators. Per RFC 7232 If-None-Match takes precedence, so it is
// checked first; If-Modified-Since (what pacman sends) is only consulted when no
// If-None-Match is present.
func notModified(req *http.Request, meta domain.FileMeta) bool {
	if inm := req.Header.Get("If-None-Match"); inm != "" {
		return meta.ETag != "" && etagMatches(inm, meta.ETag)
	}
	if ims := req.Header.Get("If-Modified-Since"); ims != "" && !meta.LastModified.IsZero() {
		if since, err := http.ParseTime(ims); err == nil {
			// Not modified when the file's mtime is at or before the client's copy;
			// truncate to whole seconds since HTTP dates have no sub-second part.
			return !meta.LastModified.Truncate(time.Second).After(since)
		}
	}
	return false
}

// etagMatches reports whether an If-None-Match header (a comma-separated list, or
// "*") contains an exact match for etag. Strong comparison only: a proxy that
// weakens the validator simply falls through to a 200, never a wrong-body 304.
func etagMatches(ifNoneMatch, etag string) bool {
	ifNoneMatch = strings.TrimSpace(ifNoneMatch)
	if ifNoneMatch == "" {
		return false
	}
	if ifNoneMatch == "*" {
		return true
	}
	for _, tok := range strings.Split(ifNoneMatch, ",") {
		if strings.TrimSpace(tok) == etag {
			return true
		}
	}
	return false
}
func (h *Handler) RepoFileListHandler(ctx *gin.Context) {
	repo := ctx.Param("repo")
	arch := ctx.Param("arch")
	l, err := h.s.RepoFileList(repo, arch)
	if err != nil {
		slog.Error("err while getting repo dir", "repo", repo, "arch", arch, "err", err)
		ctx.JSON(http.StatusInternalServerError, domain.APIError{Message: err.Error()})
		return
	}

	slog.Debug("repoHandler", "repoName", repo, "arch", arch, "filelist", l)

	ctx.HTML(http.StatusOK, "filelist.tmpl", gin.H{
		"List": l,
	})
}
