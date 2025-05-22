package localfs

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/ayato/domain"
)

func (l *LocalPkgBinaryStore) DeleteFile(repo string, arch string, file string, useSignedDB bool, gnupgDir *string) error {
	repoDir, err := l.getRepoDir(repo)
	if err != nil {
		return err
	}

	// Remove package file to the repository directory
	pkgPath := path.Join(repoDir, "x86_64", file)
	slog.Info("remove pkg file", "file", pkgPath)
	if err := os.Remove(pkgPath); err != nil {
		slog.Warn("remove pkg file err", "err", err)
	}

	return nil
}

func (l *LocalPkgBinaryStore) StoreFile(repo string, arch string, file domain.IFileSeekStream, useSignedDB bool, gnupgDir *string) error {
	repoDir, err := l.getRepoDir(repo)
	if err != nil {
		return err
	}

	repoPath := path.Join(repoDir, arch)
	if err := os.MkdirAll(repoPath, os.ModePerm); err != nil {
		return fmt.Errorf("mkdir %s err: %s", repoPath, err.Error())
	}

	dstFilePath := path.Join(repoPath, file.FileName())
	// if err := cp.Copy(file, dstFile); err != nil {
	// 	return fmt.Errorf("copy file err: %s", err.Error())
	// }
	dstFile, err := os.Create(dstFilePath)
	if err != nil {
		return fmt.Errorf("create file err: %s", err.Error())
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, file); err != nil {
		return fmt.Errorf("copy file err: %s", err.Error())
	}

	err = l.repoAdd(repo, arch, file.FileName(), useSignedDB, gnupgDir)
	if err != nil {
		return fmt.Errorf("repo-add err: %s", err.Error())
	}
	return nil
}

func (l *LocalPkgBinaryStore) Files(repo string, arch string) ([]string, error) {
	repoDir, err := l.getRepoDir("ayato")
	if err != nil {
		return nil, err
	}
	repoPath := path.Join(repoDir, arch)

	entries, err := os.ReadDir(repoPath)
	if err != nil {
		return nil, err
	}
	files := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}
