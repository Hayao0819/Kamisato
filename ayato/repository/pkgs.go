package repository

import (
	"fmt"

	remote "github.com/Hayao0819/Kamisato/alpm/remoterepo"
)

// TODO: implement this function
func (r *Repository) PkgNames(repoName, archName string) ([]string, error) {
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
	db, err := r.pkgBinStore.FetchFile(name, arch, fmt.Sprintf("%s.db.tar.gz", name))
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

func (r *Repository) RepoNames() ([]string, error) {
	names, err := r.pkgBinStore.RepoNames()
	if err != nil {
		return nil, err
	}
	return names, nil
}
