package localfs

import (
	"fmt"
	"log/slog"
	"os"
	"path"

	cp "github.com/otiai10/copy"
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

func (l *LocalPkgBinaryStore) StoreFile(repo string, arch string, file string, useSignedDB bool, gnupgDir *string) error {
	repoDir, err := l.getRepoDir(repo)
	if err != nil {
		return err
	}

	repoPath := path.Join(repoDir, arch)
	if err := os.MkdirAll(repoPath, os.ModePerm); err != nil {
		return fmt.Errorf("mkdir %s err: %s", repoPath, err.Error())
	}

	if arch != "any" {
		dstFile := path.Join(repoPath, path.Base(file))
		if err := cp.Copy(file, dstFile); err != nil {
			return fmt.Errorf("copy file err: %s", err.Error())
		}
		return nil
	}

	arches, err := l.ExistArchs(repo)
	if err != nil {
		return err
	}
	for _, arch := range arches {
		dstFile := path.Join(repoDir, arch, path.Base(file))
		if err := cp.Copy(file, dstFile); err != nil {
			return fmt.Errorf("copy file err: %s", err.Error())
		}
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
