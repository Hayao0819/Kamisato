package localfs

import (
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/ayato/repository/pacman"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/utils"
)

func (l *LocalPkgBinaryStore) repoAdd(name string, arch string, fileName string, useSignedDB bool, gnupgDir *string) error {
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

func (s *LocalPkgBinaryStore) RepoAdd(repo, arch string, pkgfile, sigfile stream.IFileSeekStream, useSignedDB bool, gnupgDir *string) error {
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

	// return  s.repoAdd(repo, arch, pkgPath, )
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
	dbfile, err := stream.OpenFileWithType(dbpath)
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
		return utils.WrapErr(err, "failed to remove repo")
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
