// SourceRepo型と関連処理
package repo

import (
	"errors"
	"log/slog"

	"github.com/Hayao0819/Kamisato/internal/conf"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/package"
	"github.com/Hayao0819/nahi/flist"
)

type SourceRepo struct {
	Config *conf.SrcRepoConfig
	Pkgs   []*pkg.Package
}

func GetSrcDirs(repodir string) ([]string, error) {
	srcdirs, err := flist.Get(repodir, flist.WithDirOnly(), flist.WithExactDepth(1))
	if err != nil {
		return nil, err
	}
	return *srcdirs, nil
}

func GetSrcRepo(repodir string) (*SourceRepo, error) {
	repo := new(SourceRepo)
	repoconfig, err := conf.LoadSrcRepoConfig(repodir)
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
		slog.Error("no src directories found", "dir", repodir)
		return nil, errors.New("no src directories found")
	}

	for _, dir := range dirs {
		slog.Info("get pkg from src", "dir", dir)
		p, err := pkg.GetPkgFromSrc(dir)
		if err != nil {
			slog.Error("get pkg from src failed", "dir", dir, "err", err)
			continue
		}
		repo.Pkgs = append(repo.Pkgs, p)
	}

	return repo, nil
}
