package localfs

import (
	"fmt"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/ayato/repository/pacman"
)

func (l *LocalPkgBinaryStore) repoAdd(name string, arch string, fileName string, useSignedDB bool, gnupgDir *string) error {
	repoDir, err := l.getRepoDir(name)
	if err != nil {
		return err
	}

	repoPath := path.Join(repoDir, arch)
	if err := os.MkdirAll(repoPath, os.ModePerm); err != nil {
		return fmt.Errorf("mkdir %s err: %s", repoPath, err.Error())
	}

	repoDbPath := path.Join(repoPath, name+".db.tar.gz")
	pkgFilePath := ""
	if fileName != "" {
		pkgFilePath = path.Join(repoPath, fileName)
	}
	if err := pacman.RepoAdd(repoDbPath, pkgFilePath, useSignedDB, gnupgDir); err != nil {
		return fmt.Errorf("repo-add err: %s", err.Error())
	}

	return nil
}

func (l *LocalPkgBinaryStore) repoRemove(name string, fileName string, useSignedDB bool, gnupgDir *string) error {
	repoDir, err := l.getRepoDir(name)
	if err != nil {
		return err
	}

	repoDbPath := path.Join(repoDir, "x86_64", name+".db.tar.gz")
	pkgFilePath := path.Join(repoDir, "x86_64", fileName)
	if err := pacman.RepoRemove(repoDbPath, pkgFilePath, useSignedDB, gnupgDir); err != nil {
		return fmt.Errorf("repo-remove err: %s", err.Error())
	}

	return nil
}

func (l *LocalPkgBinaryStore) Init(name string, arch string, useSignedDB bool, gnupgDir *string) error {
	// slog.Info("init pkg repo", "name", name)
	if err := l.repoAdd(name, arch, "", useSignedDB, gnupgDir); err != nil {
		return err
	}
	return nil
}
