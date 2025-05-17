package localfs

import (
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/repo"
)

func (l *LocalPkgBinaryStore) dbAdd(name string, fileName string, useSignedDB bool, gnupgDir *string) error {
	repoDir, err := l.getRepoDir(name)
	if err != nil {
		return err
	}

	repoPath := path.Join(repoDir, "x86_64")
	if err := os.MkdirAll(repoPath, os.ModePerm); err != nil {
		return fmt.Errorf("mkdir %s err: %s", repoPath, err.Error())
	}

	repoDbPath := path.Join(repoPath, name+".db.tar.gz")
	pkgFilePath := path.Join(repoPath, fileName)
	if err := repo.RepoAdd(repoDbPath, pkgFilePath, useSignedDB, gnupgDir); err != nil {
		return fmt.Errorf("repo-add err: %s", err.Error())
	}

	return nil
}

func (l *LocalPkgBinaryStore) dbRemove(name string, fileName string, useSignedDB bool, gnupgDir *string) error {
	repoDir, err := l.getRepoDir(name)
	if err != nil {
		return err
	}

	repoDbPath := path.Join(repoDir, "x86_64", name+".db.tar.gz")
	pkgFilePath := path.Join(repoDir, "x86_64", fileName)
	if err := repo.RepoRemove(repoDbPath, pkgFilePath, useSignedDB, gnupgDir); err != nil {
		return fmt.Errorf("repo-remove err: %s", err.Error())
	}

	return nil
}

func (l *LocalPkgBinaryStore) Init(useSignedDB bool, gnupgDir *string) error {
	names, err := l.RepoNames()
	if err != nil {
		return err
	}

	for _, name := range names {
		slog.Info("init pkg repo", "name", name)
		if err := l.dbAdd(name, "", useSignedDB, gnupgDir); err != nil {
			return err
		}
	}
	return nil
}
