package repo

import (
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/conf"
	"github.com/Hayao0819/Kamisato/internal/logger"
)

type SourceRepo struct {
	Config *conf.RepoConfig
	Pkgs   []*Package
}

func GetRepository(repodir string) (*SourceRepo, error) {
	repoconfig := new(conf.RepoConfig)
	repo := new(SourceRepo)
	if err := conf.LoadRepoConfig(repodir, repoconfig); err != nil {
		return nil, err
	}
	repo.Config = repoconfig

	dirs, err := os.ReadDir(repodir)
	if err != nil {
		return nil, err
	}
	for _, dir := range dirs {
		if dir.IsDir() {
			pkgdir := path.Join(repodir, dir.Name())
			pkg, err := GetPkgFromSrc(pkgdir)
			if err != nil {
				logger.Error(err.Error())
				continue
			}
			repo.Pkgs = append(repo.Pkgs, pkg)
		} else {
			if dir.Name() == ".SRCINFO" {
				continue
			}
		}
	}

	return repo, nil

}
