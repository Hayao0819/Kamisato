package repo

import (
	"log/slog"

	alpmpkg "github.com/Hayao0819/Kamisato/alpm/pkg"
	"github.com/Hayao0819/Kamisato/conf"
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
		slog.Error("load repo config failed", "dir", repodir, "err", err)
		return nil, err
	}
	repo.Config = repoconfig

	dirs, err := GetSrcDirs(repodir)
	if err != nil {
		return nil, err
	}

	if len(dirs) == 0 {
		slog.Info("no src directories found", "dir", repodir)
		return nil, nil
	}

	for _, dir := range dirs {
		slog.Info("get pkg from src", "dir", dir)
		pkg, err := alpmpkg.GetPkgFromSrc(dir)
		if err != nil {
			slog.Error("get pkg from src failed", "dir", dir, "err", err)
			continue
		}
		repo.Pkgs = append(repo.Pkgs, pkg)
	}

	return repo, nil

}
