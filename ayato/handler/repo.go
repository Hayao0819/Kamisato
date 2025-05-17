package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) RepoFileHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	fileName := ctx.Param("file")

	// FileServerに渡す http.StripPrefixのprefixを決定
	slog.Info("repoHandler", "repoName", repoName)
	if err := h.s.Repo(repoName, fileName, ctx.Writer, ctx.Request); err != nil {
		ctx.String(http.StatusNotFound, "failed to serve %s", repoName)
	}
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
