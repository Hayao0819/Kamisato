package service

import (
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/repo"
	cp "github.com/otiai10/copy"
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
	slog.Info("upload pkg file", "file", pkgFile)

	// Verify repository directory
	if s.repo.VerifyPkgRepo(rname) != nil {
		slog.Warn("repository directory not found", "repo", rname)
		if err := s.repo.PkgRepoInit(false, nil); err != nil {
			return fmt.Errorf("init repo err: %s", err.Error())
		}
	}

	// Store package file to the database
	// FIXME: バイナリからパッケージ名を特定する

	p, err := repo.GetPkgFromBin(pkgFile)
	if err != nil {
		return fmt.Errorf("get pkg from bin err: %s", err.Error())
	}
	slog.Info("get pkg from bin", "pkgname", p.Info().Pkgname, "pkgver", p.Info().Pkgver)

	if err := s.repo.SetPkgFileName(p.Info().Pkgname, path.Base(pkgFile)); err != nil {
		return fmt.Errorf("set pkg file err: %s", err.Error())
	}

	// Move package file to the repository directory
	pkgPath := path.Join(repoDir, "x86_64", path.Base(pkgFile))
	if err := os.MkdirAll(path.Dir(pkgPath), os.ModePerm); err != nil {
		return fmt.Errorf("create dir err: %s", err.Error())
	}
	// if err := utils.MoveFile(pkgFile, path.Join(repoDir, "x86_64", path.Base(pkgFile))); err != nil {
	// 	return fmt.Errorf("move file err: %s", err.Error())
	// }

	if err := cp.Copy(pkgFile, pkgPath); err != nil {
		return fmt.Errorf("copy file err: %s", err.Error())
	}

	// Update the package database
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
