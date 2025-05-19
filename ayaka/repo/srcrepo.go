package repo

import (
	alpmpkg "github.com/Hayao0819/Kamisato/alpm/pkg"
	"github.com/Hayao0819/Kamisato/conf"
	"github.com/Hayao0819/Kamisato/internal/logger"
	"github.com/Hayao0819/nahi/flist"
)

type SourceRepo struct {
	Config *conf.RepoConfig
	Pkgs   []*alpmpkg.Package
}

func GetSrcDirs(repodir string) ([]string, error) {
	srcdirs, err := flist.Get(repodir, flist.WithDirOnly(), flist.WithExactDepth(1))
	if err != nil {
		return nil, err
	}
	return *srcdirs, nil
}

func GetSrcRepo(repodir string) (*SourceRepo, error) {
	// repoconfig := new(conf.RepoConfig)
	repo := new(SourceRepo)
	repoconfig, err := conf.LoadRepoConfig(repodir)
	if err != nil {
		return nil, err
	}
	repo.Config = repoconfig

	dirs, err := GetSrcDirs(repodir)
	if err != nil {
		return nil, err
	}

	for _, dir := range dirs {
		pkg, err := alpmpkg.GetPkgFromSrc(dir)
		if err != nil {
			logger.Error(err.Error())
			continue
		}
		repo.Pkgs = append(repo.Pkgs, pkg)
	}

	return repo, nil

}
