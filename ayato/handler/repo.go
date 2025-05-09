package handler

import (
	"net/http"
	"path"

	"github.com/gin-gonic/gin"
)

func (h *Handler) RepoHandler(ctx *gin.Context) {
	repoName := ctx.Param("repo")

	// Assemble the file path
	var repoDbPath string // Path to the repository database
	for _, r := range h.cfg.RepoPath {
		if path.Base(r) == repoName {
			repoDbPath = r
			break
		}
	}

	// FileServerに渡す http.StripPrefixのprefixを決定
	prefix := "/repo/" + repoName

	// FileServerハンドラー作成
	fileServer := http.StripPrefix(prefix, http.FileServer(http.Dir(repoDbPath)))

	// Ginのcontextから http.ResponseWriter/Request を使って FileServer呼び出し
	fileServer.ServeHTTP(ctx.Writer, ctx.Request)

}
