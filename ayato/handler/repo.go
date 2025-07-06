package handler

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RepoFileHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")
	arch := ctx.Param("arch")
	fileName := ctx.Param("file")

	slog.Info("repoHandler", "repoName", repoName)

	s, err := h.s.GetFile(repoName, arch, fileName)
	if err != nil {
		ctx.JSON(http.StatusNotFound, domain.APIError{
			Message: "failed to serve " + fileName,
			Reason:  err,
		})
		return
	}
	defer s.Close()

	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", s.FileName()))
	ctx.Header("Content-Type", s.ContentType())

	// 通常のファイル送信として出力
	_, err = io.Copy(ctx.Writer, s)
	if err != nil {
		slog.Error("failed to write file to response", "error", err)
	}
}
func (h *Handler) RepoFileListHandler(ctx *gin.Context) {
	// slog.Info("repoHandler", "repoName", ctx.Param("repo"))
	repo := ctx.Param("repo")
	arch := ctx.Param("arch")
	// fmt.Println("hoge")
	l, err := h.s.RepoFileList(repo, arch)
	if err != nil {
		slog.Error("err while getting repo dir", "repo", repo, "arch", arch, "err", err)
	}

	slog.Warn("repoHandler", "repoName", repo, "arch", arch, "filelist", l)

	ctx.HTML(http.StatusOK, "filelist.tmpl", gin.H{
		"List": l,
	})
}
