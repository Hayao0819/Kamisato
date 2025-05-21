package handler

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) RepoFileHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	arch := ctx.Param("arch")
	fileName := ctx.Param("file")

	// FileServerに渡す http.StripPrefixのprefixを決定
	slog.Info("repoHandler", "repoName", repoName)

	s, err := h.s.GetFile(repoName, arch, fileName)
	if err != nil {
		ctx.String(http.StatusNotFound, "failed to serve %s", repoName)
	}

	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", s.FileName()))
	ctx.Header("Content-Type", s.ContentType())
	ctx.Stream(func(w io.Writer) bool {
		_, err := io.Copy(w, s)
		return err == nil
	})
}

func (h *Handler) RepoFileListHandler(ctx *gin.Context) {
	repo := ctx.Param("repo")
	arch := ctx.Param("arch")
	l, err := h.s.RepoFileList(repo, arch)
	if err != nil {
		slog.Error("err while getting repo dir", "repo", repo, "arch", arch, "err", err)
	}

	ctx.HTML(http.StatusOK, "repolist.tmpl", gin.H{
		"List": l,
	})

}

func (h *Handler) RepoListHandler(ctx *gin.Context) {
	l, err := h.s.RepoList()
	if err != nil {
		slog.Error("err while getting repo dir", "err", err)
		ctx.String(http.StatusInternalServerError, "err while getting repo dir")
		return
	}

	ctx.HTML(http.StatusOK, "repolist.tmpl", gin.H{
		"List": l,
	})
}
