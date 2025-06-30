package repository

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/pkg/pacman/remote"
)

func (r *Repository) PkgNames(repoName, archName string) ([]string, error) {
	// FIXME: リクエスト来るたびに毎回DBを開くのは非効率
	db, err := r.pkgBinStore.FetchFile(repoName, archName, fmt.Sprintf("%s.db.tar.gz", repoName))
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rr, err := remote.GetRepo(repoName, db)
	if err != nil {
		return nil, err
	}
	if rr == nil {
		return nil, fmt.Errorf("failed to get repository")
	}
	if len(rr.Pkgs) == 0 {
		return nil, fmt.Errorf("no packages found in the repository")
	}
	pkgs := make([]string, 0)
	for _, pkg := range rr.Pkgs {
		pi := pkg.MustPKGINFO()
		pkgs = append(pkgs, pi.PkgBase)
	}
	return pkgs, nil
}

func (r *Repository) RemoteRepo(name, arch string) (*remote.RemoteRepo, error) {
	db, err := r.FetchDB(name, arch)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rr, err := remote.GetRepo(name, db)
	if err != nil {
		return nil, err
	}
	if rr == nil {
		return nil, fmt.Errorf("failed to get repository")
	}
	return rr, nil
}

func (r *Repository) PkgFiles(repoName, archName, pkgName string) ([]string, error) {
	db, err := r.FetchDB(repoName, archName)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rr, err := remote.GetRepo(repoName, db)
	if err != nil {
		return nil, err
	}
	if rr == nil {
		return nil, fmt.Errorf("failed to get repository")
	}

	return nil, nil

	// TODO: Implement this function

	// pkg := rr.GetPkg(pkgName)
	// if pkg == nil {
	// 	return nil, fmt.Errorf("package %s not found in repository %s", pkgName, repoName)
	// }

	// files := make([]string, 0)
	// for _, file := range pkg.Files {
	// 	files = append(files, file.Name)
	// }
	// return files, nil
}
