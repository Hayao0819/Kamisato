// SourceRepo型と関連処理
package repo

import (
	"errors"
	"io/fs"
	"log/slog"
	"path/filepath"

	"github.com/Hayao0819/Kamisato/internal/conf"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/package"
	"github.com/Hayao0819/nahi/futils"
)

type SourceRepo struct {
	Config *conf.SrcRepoConfig
	Pkgs   []*pkg.Package
}

func GetSrcDirs(repodir string) ([]string, error) {
	slog.Debug("get src dirs", "dir", repodir)
	// srcdirs, err := flist.Get(repodir, flist.WithDirOnly(), flist.WithExactDepth(0))
	srcdirs := []string{}
	err := filepath.WalkDir(repodir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}

		if futils.Exists(filepath.Join(path, "PKGBUILD")) && futils.Exists(filepath.Join(path, ".SRCINFO")) {
			slog.Debug("found src dir", "dir", path)
			srcdirs = append(srcdirs, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return srcdirs, nil
}

func GetSrcRepo(repodir string) (*SourceRepo, error) {
	repo := new(SourceRepo)
	repoconfig, err := conf.LoadSrcRepoConfig(repodir)
	if err != nil {
		slog.Error("load repo config failed", "dir", repodir, "err", err)
		return nil, err
	}
	repo.Config = repoconfig
	slog.Debug("loaded repo config", "dir", repodir, "config", repoconfig)

	dirs, err := GetSrcDirs(repodir)
	if err != nil {
		return nil, err
	}

	if len(dirs) == 0 {
		slog.Error("no src directories found", "dir", repodir)
		return nil, errors.New("no src directories found")
	}

	for _, dir := range dirs {
		// slog.Info("get pkg from src", "dir", dir)
		p, err := pkg.PkgFromSrc(dir)
		if err != nil {
			slog.Error("get pkg from src failed", "dir", dir, "err", err)
			continue
		}
		repo.Pkgs = append(repo.Pkgs, p)
	}

	return repo, nil
}
