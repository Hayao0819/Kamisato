package service

import (
	"fmt"
	"path"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/repo"
)

// TODO: ディレクトリのハードコーディングをやめる
func (s *Service) UploadPkgFile(rname string, name [2]string) error {
	// Correct information
	repoDir, err := s.repo.PkgRepoDir(rname)
	if err != nil {
		// return fmt.Errorf("repo %s not found", repo)
		return err
	}
	pkgFile := name[0]
	// sigFile := name[1]

	// Move package file to the repository directory
	pkgPath := path.Join(repoDir, "x86_64", path.Base(pkgFile))
	if err := utils.MoveFile(pkgFile, path.Join(repoDir, "x86_64")); err != nil {
		return fmt.Errorf("move file err: %s", err.Error())
	}

	useSignedDB := false
	var gnupgDir *string // TODO: Check if the directory exists
	repoDbPath := path.Join(repoDir, "x86_64", rname+".db.tar.gz")
	err = repo.RepoAdd(repoDbPath, pkgPath, useSignedDB, gnupgDir)
	if err != nil {
		return fmt.Errorf("repo-add err: %s", err.Error())
	}

	// 一時ディレクトリにあるファイルの削除は呼び出し元の責務
	// この関数の副作用はリポジトリディレクトリ以下のみ
	return nil
}
