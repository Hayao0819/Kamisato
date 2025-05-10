package service

import (
	"fmt"
	"path"

	"github.com/Hayao0819/Kamisato/repo"
)

func (s *Service) UploadPkgFile(rname string, name [2]string) error {
	repoDir, err := s.repo.RepoDir(rname)
	if err != nil {
		// return fmt.Errorf("repo %s not found", repo)
		return err
	}

	pkgFile := name[0]
	// sigFile := name[1]

	// fullPkgBinary := path.Join(repoDir, file.Filename)

	useSignedDB := false
	var gnupgDir *string // TODO: Check if the directory exists
	repoDbPath := path.Join(repoDir, "x86_64", rname+".db.tar.gz")
	err = repo.RepoAdd(repoDbPath, pkgFile, useSignedDB, gnupgDir)
	if err != nil {
		return fmt.Errorf("repo-add err: %s", err.Error())
	}

	return nil
}
