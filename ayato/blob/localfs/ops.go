package localfs

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"strings"

	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/nahi/flist"
	"github.com/Hayao0819/nahi/futils"
	"github.com/samber/lo"
)

// validatePathComponent rejects values that could escape the repo directory.
func validatePathComponent(c string) error {
	if c == "" || c == "." || strings.ContainsRune(c, '/') || strings.ContainsRune(c, os.PathSeparator) || strings.Contains(c, "..") {
		return os.ErrNotExist
	}
	return nil
}

func (l *LocalStore) StoreFile(repo string, arch string, file stream.SeekFile) error {
	repoDir, err := l.getRepoDir(repo)
	if err != nil {
		return err
	}

	if err := validatePathComponent(arch); err != nil {
		return err
	}
	name := path.Base(file.FileName())
	if err := validatePathComponent(name); err != nil {
		return err
	}

	repoPath := path.Join(repoDir, arch)
	if err := os.MkdirAll(repoPath, os.ModePerm); err != nil {
		return utils.WrapErr(err, fmt.Sprintf("mkdir %s err", repoPath))
	}

	dstFilePath := path.Join(repoPath, name)
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

	if err := validatePathComponent(arch); err != nil {
		return nil, err
	}
	if err := validatePathComponent(file); err != nil {
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

	if err := validatePathComponent(arch); err != nil {
		return err
	}
	if err := validatePathComponent(file); err != nil {
		return err
	}

	pkgPath := path.Join(repoDir, arch, file)
	slog.Info("remove pkg file", "file", pkgPath)
	if err := os.Remove(pkgPath); err != nil {
		slog.Warn("remove pkg file err", "err", err)
		return utils.WrapErr(err, "failed to remove pkg file")
	}

	return nil
}

func (l *LocalStore) RepoNames() ([]string, error) {
	if l.repoDir == "" {
		return nil, errors.New("local repository directory is not set")
	}

	dirs, err := flist.Get(l.repoDir, flist.WithDirOnly(), flist.WithExactDepth(1))
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
	repoDir, err := l.getRepoDir(repo)
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
