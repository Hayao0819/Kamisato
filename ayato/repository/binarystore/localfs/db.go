package localfs

import (
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/ayato/repository/pacman"
	domain "github.com/Hayao0819/Kamisato/ayato/stream"
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

func (s *LocalPkgBinaryStore) RepoAdd(repo, arch string, pkgfile, sigfile domain.IFileSeekStream, useSignedDB bool, gnupgDir *string) error {
	t, err := os.MkdirTemp("", "ayato-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(t)

	pkgPath, err := writeStreamToFile(t, pkgfile)
	if err != nil {
		return err
	}

	repoDir, err := s.getRepoDir(repo)
	if err != nil {
		return err
	}
	repoDir = path.Join(repoDir, arch)

	dbpath := path.Join(repoDir, repo+".db.tar.gz")
	dbfile, err := domain.OpenFileStreamWithTypeDetection(dbpath)
	if err != nil {
		// if s3shared.
		return fmt.Errorf("failed to open file %s: %w", dbpath, err)
	}
	defer dbfile.Close()

	dbPath, err := writeStreamToFile(t, dbfile)
	if err != nil {
		return err
		// slog.Error("writeStreamToFile", "err", err)
	}

	return pacman.RepoAdd(dbPath, pkgPath, useSignedDB, gnupgDir)
}

func (s *LocalPkgBinaryStore) RepoRemove(repo string, arch string, pkg string, useSignedDB bool, gnupgDir *string) error {
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
	dbfile, err := domain.OpenFileStreamWithTypeDetection(dbpath)
	if err != nil {
		// if s3shared.
		return fmt.Errorf("failed to open file %s: %w", dbpath, err)
	}
	defer dbfile.Close()

	dbPath, err := writeStreamToFile(t, dbfile)
	if err != nil {
		return err
	}

	if err := pacman.RepoRemove(dbPath, pkg, useSignedDB, gnupgDir); err != nil {
		slog.Error("RepoRemove", "err", err)
		return err
	}

	return nil
}

func (l *LocalPkgBinaryStore) Init(name string, arch string, useSignedDB bool, gnupgDir *string) error {
	// slog.Info("init pkg repo", "name", name)
	slog.Debug("init pkg repo", "name", name, "arch", arch)
	if err := l.repoAdd(name, arch, "", useSignedDB, gnupgDir); err != nil {
		return err
	}
	return nil
}
