package repo

import (
	"errors"
	"io/fs"
	"log/slog"
	"path/filepath"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/nahi/futils"
)

// SrcConfig mirrors conf.SrcRepoConfig fields to keep the domain layer free of the conf package; callers pass it into GetSrcRepo.
type SrcConfig struct {
	Name        string
	Maintainer  string
	URL         string
	Build       builder.ProjectConfig
	InstallPkgs struct {
		Files []string
		Names []string
	}
}

type SourceRepo struct {
	Config  *SrcConfig
	Pkgs    []*pkg.SourcePackage
	Dir     string
	DestDir string
}

func GetSrcDirs(repodir string) ([]string, error) {
	slog.Debug("get src dirs", "dir", repodir)
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

func GetSrcRepo(repodir string, cfg *SrcConfig) (*SourceRepo, error) {
	repo := new(SourceRepo)
	if cfg == nil {
		return nil, errors.New("source repo config is nil")
	}
	repo.Config = cfg
	slog.Debug("loaded repo config", "dir", repodir, "config", cfg)

	dirs, err := GetSrcDirs(repodir)
	if err != nil {
		return nil, err
	}

	if len(dirs) == 0 {
		slog.Error("no src directories found", "dir", repodir)
		return nil, errors.New("no src directories found")
	}

	for _, dir := range dirs {
		p, err := pkg.OpenSourcePackage(dir)
		if err != nil {
			slog.Error("get pkg from src failed", "dir", dir, "err", err)
			continue
		}
		repo.Pkgs = append(repo.Pkgs, p)
	}

	return repo, nil
}
