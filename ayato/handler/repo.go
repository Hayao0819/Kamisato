package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) RepoHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")

	// FileServerに渡す http.StripPrefixのprefixを決定

	if err := h.s.Repo(ctx, repoName); err != nil {
		ctx.String(http.StatusNotFound, "failed to serve %s", repoName)
	}
}
