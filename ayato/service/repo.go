package service

import (
	"net/http"
)

func (s *Service) Repo(repo string, file string, w http.ResponseWriter, req *http.Request) error {

	// repoDbpath, err := s.repo.PkgRepoDir(repo)
	// if err != nil {
	// 	// s.logger.Errorf("repo %s not found", repo)
	// 	return err
	// }

	// // FileServerハンドラー作成
	// fileServer := http.StripPrefix(handlerName, http.FileServer(http.Dir(repoDbpath)))

	// // Ginのcontextから http.ResponseWriter/Request を使って FileServer呼び出し
	// fileServer.ServeHTTP(ctx.Writer, ctx.Request)
	// return nil

	// fileToServe := path.Join(repoDbpath, "x86_64", file)
	// http.ServeFile(w, req, fileToServe)
	return nil

}

func (s *Service) RepoList() ([]string, error) {
	return s.r.Repos()

}
func (s *Service) RepoFileList(repo, arch string) ([]string, error) {
	return s.r.Files(repo, arch)
}
