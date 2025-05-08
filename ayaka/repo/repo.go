package repo

import (
	"os"
	"path"

	builder "github.com/Hayao0819/Kamisato/ayaka/abs"
	"github.com/Hayao0819/Kamisato/conf"
	"github.com/Hayao0819/Kamisato/internal/logger"
	"github.com/Morganamilo/go-srcinfo"
)

type Repository struct {
	Config *conf.RepoConfig
	Pkgs   []*Package
}

// func (r *Repository) DestDir() (string, error) {
// 	c, err := conf.LoadAyakaConfig()
// 	if err != nil {
// 		return "", err
// 	}
// 	dstdir := path.Join(c.DestDir, r.Config.Name)
// 	return dstdir, nil
// }

func (r *Repository) Build(t *builder.Target, dest string) error {
	dstdir := path.Join(dest, t.Arch)
	if err := os.MkdirAll(dstdir, 0755); err != nil {
		return err
	}
	for _, pkg := range r.Pkgs {
		if err := pkg.Build("archbuild", t, dest); err != nil {
			logger.Error(err.Error())
		}

		/*
			if err := r.UploadToBlinky(conf.AppConfig.BlinkyServer, pkg); err != nil {
				logger.Error(err.Error())
			}
		*/

	}
	return nil
}

func (r *Repository) UploadAllPackageToBlinky(server string) error {
	for _, pkg := range r.Pkgs {
		if err := pkg.UploadToBlinky(server, r); err != nil {
			return err
		}
	}
	return nil
}

func GetRepository(repodir string) (*Repository, error) {
	repoconfig := new(conf.RepoConfig)
	repo := new(Repository)
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
			pkg, err := GetPackage(pkgdir)
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

func GetPackage(dir string) (*Package, error) {
	info, err := srcinfo.ParseFile(path.Join(dir, ".SRCINFO"))
	if err != nil {
		return nil, err
	}

	pkg := new(Package)
	pkg.Path = dir
	pkg.Srcinfo = info

	return pkg, nil
}
