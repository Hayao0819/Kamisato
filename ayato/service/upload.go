package service

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/ayato/utils"
)

func (s *Service) UploadPkgFile(repo string, name [2]string) error {
	repoDir, err := s.repo.RepoDir(repo)
	if err != nil {
		return fmt.Errorf("repo %s not found", repo)
	}

	pkgFile := name[0]
	// sigFile := name[1]

	// fullPkgBinary := path.Join(repoDir, file.Filename)

	useSignedDB := false
	var gnupgDir *string // TODO: Check if the directory exists
	err = utils.RepoAdd(repoDir, pkgFile, useSignedDB, gnupgDir)
	if err != nil {
		return fmt.Errorf("repo-add err: %s", err.Error())
	}

	return nil
}
