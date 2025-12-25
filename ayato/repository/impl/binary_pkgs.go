package impl

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/pkg/pacman/remote"
)

// PkgNames returns all package base names in the repository.
// FIXME: Inefficient to open DB every time. Optimization with cache, etc. is recommended.
func (r *PackageBinaryRepository) PkgNames(repoName, archName string) ([]string, error) {
	db, err := r.pkgBinStore.FetchFile(repoName, archName, fmt.Sprintf("%s.db.tar.gz", repoName))
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rr, err := remote.Repo(repoName, db)
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

// RemoteRepo returns the remote repository object for the specified repository name and architecture.
func (r *PackageBinaryRepository) RemoteRepo(name, arch string) (*remote.RemoteRepo, error) {
	db, err := r.FetchDB(name, arch)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rr, err := remote.Repo(name, db)
	if err != nil {
		return nil, err
	}
	if rr == nil {
		return nil, fmt.Errorf("failed to get repository")
	}
	return rr, nil
}

// PkgFiles returns a list of package files in the repository.
// TODO: Not yet implemented (get list of package files)
func (r *PackageBinaryRepository) PkgFiles(repoName, archName, pkgName string) ([]string, error) {
	db, err := r.FetchDB(repoName, archName)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rr, err := remote.Repo(repoName, db)
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
