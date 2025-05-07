package repo

import (
	"os"
	"path"

	builder "github.com/Hayao0819/Kamisato/abs"
	"github.com/Hayao0819/Kamisato/conf"
	"github.com/Hayao0819/Kamisato/logger"
	"github.com/Morganamilo/go-srcinfo"
	"github.com/spf13/viper"
)

type Repository struct {
	Config *conf.RepoConf
	Pkgs   []*Package
}

func (r *Repository) GetDistDir() string {
	return path.Join(conf.AppConfig.DistDir, r.Config.Name)
}

func (r *Repository) Build(t *builder.Target) error {
	dstdir := path.Join(r.GetDistDir(), t.Arch)
	if err := os.MkdirAll(dstdir, 0755); err != nil {
		return err
	}
	for _, pkg := range r.Pkgs {
		if err := pkg.Build("archbuild", t); err != nil {
			logger.Error(err.Error())
		}

		if err := pkg.MovePkgFile(dstdir); err != nil {
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

func Get() (*Repository, error) {
	repodir := viper.GetString("repodir")
	repoconfig := new(conf.RepoConf)
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
			info, err := srcinfo.ParseFile(path.Join(repodir, dir.Name(), ".SRCINFO"))
			if err != nil {
				return nil, err
			}

			pkg := new(Package)
			pkg.Path = path.Join(repodir, dir.Name())
			pkg.Srcinfo = info
			repo.Pkgs = append(repo.Pkgs, pkg)
		}
	}

	return repo, nil

}
