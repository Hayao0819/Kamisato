package localfs

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"strings"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
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
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		return errwrap.WrapErr(err, fmt.Sprintf("mkdir %s err", repoPath))
	}

	dstFilePath := path.Join(repoPath, name)
	dstFile, err := os.OpenFile(dstFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return errwrap.WrapErr(err, "failed to create file")
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, file); err != nil {
		return errwrap.WrapErr(err, "failed to copy file")
	}
	return nil
}

func (l *LocalStore) StoreFileWithSignedURL(repo string, arch string, name string) (string, error) {
	return "", nil
}

// FetchFileWithETag returns the file with an empty version token: localfs has no
// object versioning.
func (l *LocalStore) FetchFileWithETag(repo, arch, file string) (stream.File, string, error) {
	f, err := l.FetchFile(repo, arch, file)
	return f, "", err
}

// FetchFileWithMeta returns the file with its on-disk modification time (the ETag
// stays empty; localfs has no object versioning). The mtime lets a pacman client
// answer its own If-Modified-Since and skip an unchanged .db. The path components
// were already validated by FetchFile, so re-stat is safe; any stat error just
// leaves LastModified zero (no conditional, full body).
func (l *LocalStore) FetchFileWithMeta(repo, arch, file string) (stream.File, blob.FileMeta, error) {
	f, err := l.FetchFile(repo, arch, file)
	if err != nil {
		return nil, blob.FileMeta{}, err
	}
	var meta blob.FileMeta
	if repoDir, derr := l.getRepoDir(repo); derr == nil {
		if fi, serr := os.Stat(path.Join(repoDir, arch, file)); serr == nil {
			meta.LastModified = fi.ModTime()
		}
	}
	return f, meta, nil
}

// StoreFileIfMatch ignores etag and stores: localfs is single-node with no object
// versioning, and the repository's per-(repo, arch) lock already serializes its
// writes within the one process that owns the directory.
func (l *LocalStore) StoreFileIfMatch(repo, arch string, file stream.SeekFile, _ string) error {
	return l.StoreFile(repo, arch, file)
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
		return nil, blob.ErrNotFound
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
		return errwrap.WrapErr(err, "failed to remove pkg file")
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
