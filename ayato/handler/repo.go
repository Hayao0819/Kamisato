package handler

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/domain"
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

func (h *Handler) RepoDetailHandler(ctx *gin.Context) {
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
	ctx.JSON(http.StatusOK, gin.H{"name": repoName, "arches": archNames})
}

func (h *Handler) RepoFileHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	arch := ctx.Param("arch")
	fileName := ctx.Param("file")

	slog.Debug("repoHandler", "repoName", repoName)

	// Redirect to a presigned GET when the backend can presign, so package bytes never
	// transit ayato (Cloud Run bills egress); a backend that cannot (localfs) returns
	// "" and we fall through to streaming.
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
	// pacman uses Last-Modified/If-Modified-Since, HTTP caches use ETag/If-None-Match.
	if meta.ETag != "" {
		ctx.Header("ETag", meta.ETag)
	}
	if !meta.LastModified.IsZero() {
		ctx.Header("Last-Modified", meta.LastModified.UTC().Format(http.TimeFormat))
	}
	// A package archive is content-immutable (version is in the name) so it caches
	// forever; the .db/.files metadata is rewritten in place so it must revalidate.
	if isImmutablePackageFile(fileName) {
		ctx.Header("Cache-Control", "public, max-age=31536000, immutable")
	} else if meta.ETag != "" || !meta.LastModified.IsZero() {
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

// isImmutablePackageFile reports whether name is a package archive or its detached
// signature; the ".pkg.tar." infix (absent from .db/.files metadata) distinguishes them.
func isImmutablePackageFile(name string) bool {
	return strings.Contains(name, ".pkg.tar.")
}

// notModified checks the request's conditional headers against the file's validators.
// Per RFC 7232 If-None-Match takes precedence; If-Modified-Since (what pacman sends)
// is consulted only when If-None-Match is absent.
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

// etagMatches reports whether an If-None-Match header (comma-separated, or "*")
// exactly matches etag. Strong comparison only, so a weakened validator falls
// through to a 200 rather than a wrong-body 304.
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
