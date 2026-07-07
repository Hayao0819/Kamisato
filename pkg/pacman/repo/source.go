package repo

import (
	"errors"
	"io/fs"
	"log/slog"
	"path/filepath"

	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/nahi/futils"
)

// SrcConfig is the source-repository configuration this package needs. It mirrors
// the fields of conf.SrcRepoConfig so the domain layer stays free of the conf
// package; callers (which already load the config) pass it into GetSrcRepo.
type SrcConfig struct {
	Name        string
	Maintainer  string
	URL         string
	Build       BuildConfig
	InstallPkgs struct {
		Files []string
		Names []string
	}
}

type BuildRepo struct{ Name, Server, SigLevel string }

type MakepkgSettings struct {
	Packager     string
	Microarch    string
	CFlagsAppend string
	Options      []string
}

type BuildConfig struct {
	Repos     []BuildRepo
	Makepkg   MakepkgSettings
	ArchBuild string
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
