package localfs

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/ayato/repository/pacman"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/nahi/flist"
	"github.com/Hayao0819/nahi/futils"
	"github.com/samber/lo"
)

// --- File operations ---

func (l *LocalStore) StoreFile(repo string, arch string, file stream.SeekFile) error {
	repoDir, err := l.getRepoDir(repo)
	if err != nil {
		return err
	}

	repoPath := path.Join(repoDir, arch)
	if err := os.MkdirAll(repoPath, os.ModePerm); err != nil {
		return utils.WrapErr(err, fmt.Sprintf("mkdir %s err", repoPath))
	}

	dstFilePath := path.Join(repoPath, file.FileName())
	dstFile, err := os.Create(dstFilePath)
	if err != nil {
		return fmt.Errorf("create file err: %s", err.Error())
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, file); err != nil {
		return fmt.Errorf("copy file err: %s", err.Error())
	}
	return nil
}

func (l *LocalStore) StoreFileWithSignedURL(repo string, arch string, name string) (string, error) {
	return "", nil
}

func (l *LocalStore) FetchFile(repo string, arch string, file string) (stream.File, error) {
	repoDir, err := l.getRepoDir(repo)
	if err != nil {
		return nil, err
	}

	pkgPath := path.Join(repoDir, arch, file)
	if !futils.Exists(pkgPath) {
		return nil, os.ErrNotExist
	}
	slog.Info("fetch pkg file", "file", pkgPath)

	streamFile, err := stream.OpenFileWithType(pkgPath)
	if err != nil {
		return nil, err
	}
	return streamFile, nil
}

func (l *LocalStore) DeleteFile(repo string, arch string, file string) error {
	repoDir, err := l.getRepoDir(repo)
	if err != nil {
		return err
	}

	// FIXME: arch がハードコードされている（x86_64 固定）。
	pkgPath := path.Join(repoDir, "x86_64", file)
	slog.Info("remove pkg file", "file", pkgPath)
	if err := os.Remove(pkgPath); err != nil {
		slog.Warn("remove pkg file err", "err", err)
		return utils.WrapErr(err, "failed to remove pkg file")
	}

	return nil
}

// --- Repository listing ---

func (l *LocalStore) RepoNames() ([]string, error) {
	if l.cfg.Store.LocalRepoDir == "" {
		return nil, errors.New("local repository directory is not set")
	}

	dirs, err := flist.Get(l.cfg.Store.LocalRepoDir, flist.WithDirOnly(), flist.WithExactDepth(1))
	if err != nil {
		return nil, err
	}
	return lo.Map(*dirs, func(item string, index int) string {
		return path.Base(item)
	}), nil
}

func (l *LocalStore) Arches(repo string) ([]string, error) {
	repoDir, err := l.getRepoDir(repo)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return nil, err
	}

	archs := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			archs = append(archs, entry.Name())
		}
	}
	return archs, nil
}

func (l *LocalStore) Files(repo string, arch string) ([]string, error) {
	// FIXME: getRepoDir("ayato") がハードコードされている（repo 引数を使うべき）。
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

// --- DB operations ---

func (l *LocalStore) repoAdd(name string, arch string, fileName string, useSignedDB bool, gnupgDir *string) error {
	repoDir, err := l.getRepoDir(name)
	if err != nil {
		return err
	}
	repoPath := path.Join(repoDir, arch)

	slog.Info("repoAdd", "repoPath", repoPath, "name", name, "arch", arch, "fileName", fileName, "useSignedDB", useSignedDB)

	if err := os.MkdirAll(repoPath, os.ModePerm); err != nil {
		return fmt.Errorf("mkdir %s err: %s", repoPath, err.Error())
	}

	repoDbPath := path.Join(repoPath, name+".db.tar.gz")
	pkgFilePath := ""
	if fileName != "" {
		pkgFilePath = path.Join(repoPath, fileName)
	}
	if err := pacman.RepoAdd(repoDbPath, pkgFilePath, useSignedDB, gnupgDir); err != nil {
		slog.Error("repoAdd", "err", err)
		return fmt.Errorf("repo-add err: %s", err.Error())
	}

	return nil
}

func (s *LocalStore) RepoAdd(repo, arch string, pkgfile, sigfile stream.SeekFile, useSignedDB bool, gnupgDir *string) error {
	t, err := os.MkdirTemp("", "ayato-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(t)

	pkgPath, err := writeStreamToFile(t, pkgfile)
	if err != nil {
		return err
	}

	_, err = writeStreamToFile(t, sigfile)
	if err != nil {
		return err
	}

	repoDir, err := s.getRepoDir(repo)
	if err != nil {
		return err
	}

	dbpath := path.Join(repoDir, arch, repo+".db.tar.gz")
	return pacman.RepoAdd(dbpath, pkgPath, useSignedDB, gnupgDir)
}

func (s *LocalStore) RepoRemove(repo string, arch string, pkg string, useSignedDB bool, gnupgDir *string) error {
	t, err := os.MkdirTemp("", "ayato-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(t)

	repoDir, err := s.getRepoDir(repo)
	if err != nil {
		return err
	}
	repoDir = path.Join(repoDir, arch)

	dbpath := path.Join(repoDir, repo+".db.tar.gz")
	dbfile, err := stream.OpenFileWithType(dbpath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", dbpath, err)
	}
	defer dbfile.Close()

	dbPath, err := writeStreamToFile(t, dbfile)
	if err != nil {
		return err
	}

	if err := pacman.RepoRemove(dbPath, pkg, useSignedDB, gnupgDir); err != nil {
		slog.Error("RepoRemove", "err", err)
		return utils.WrapErr(err, "failed to remove repo")
	}

	return nil
}

func (l *LocalStore) InitArch(name string, arch string, useSignedDB bool, gnupgDir *string) error {
	slog.Debug("init pkg repo", "name", name, "arch", arch)
	if err := l.repoAdd(name, arch, "", useSignedDB, gnupgDir); err != nil {
		return err
	}
	return nil
}
