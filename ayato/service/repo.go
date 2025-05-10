package service

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Service) Repo(ctx *gin.Context, repo string) error {

	repoDbpath, err := s.repo.RepoDir(repo)
	if err != nil {
		// s.logger.Errorf("repo %s not found", repo)
		return err
	}

	handlerName := "/repo/" + repo

	// FileServerハンドラー作成
	fileServer := http.StripPrefix(handlerName, http.FileServer(http.Dir(repoDbpath)))

	// Ginのcontextから http.ResponseWriter/Request を使って FileServer呼び出し
	fileServer.ServeHTTP(ctx.Writer, ctx.Request)
	return nil

}
