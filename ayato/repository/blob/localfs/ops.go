package localfs

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"sync"
	"syscall"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/nahi/flist"
	"github.com/Hayao0819/nahi/futils"
	"github.com/samber/lo"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

func (l *LocalStore) StoreFile(repo string, arch string, file stream.SeekFile) error {
	repoPath, dstFilePath, err := l.prepareObjectPath(repo, arch, path.Base(file.FileName()))
	if err != nil {
		return err
	}
	return withObjectLock(repoPath, path.Base(file.FileName()), func() error {
		return writeAtomicFile(repoPath, dstFilePath, file)
	})
}

func (l *LocalStore) prepareObjectPath(repo, arch, name string) (repoPath, objectPath string, err error) {
	repoDir, err := l.getRepoDir(repo)
	if err != nil {
		return "", "", err
	}
	if err := blob.ValidatePathComponent(arch); err != nil {
		return "", "", err
	}
	if err := blob.ValidatePathComponent(name); err != nil {
		return "", "", err
	}

	repoPath = path.Join(repoDir, arch)
	if err := os.MkdirAll(repoPath, 0o755); err != nil { //nolint:gosec // published pacman repo dir is world-readable by design
		return "", "", errors.WrapErr(err, fmt.Sprintf("mkdir %s err", repoPath))
	}
	return repoPath, path.Join(repoPath, name), nil
}

func withObjectLock(repoPath, name string, operation func() error) error {
	lockDir := path.Join(repoPath, ".locks")
	if err := os.MkdirAll(lockDir, 0o755); err != nil { //nolint:gosec // lock directory contains no secrets
		return errors.WrapErr(err, "create object lock directory")
	}
	lockName := fmt.Sprintf("%x.lock", sha256.Sum256([]byte(name)))
	lock, err := os.OpenFile(path.Join(lockDir, lockName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return errors.WrapErr(err, "open object lock")
	}
	defer func() { _ = lock.Close() }()
	fd := int(lock.Fd()) //nolint:gosec // Unix file descriptors fit in int on every supported target.
	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		return errors.WrapErr(err, "lock object")
	}
	defer func() { _ = syscall.Flock(fd, syscall.LOCK_UN) }()
	return operation()
}

// LockPublication obtains the repository publication lock.
func (l *LocalStore) LockPublication(repo string) (func(), error) {
	repoPath, err := l.getRepoDir(repo)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(repoPath, 0o755); err != nil { //nolint:gosec // published repository directory
		return nil, errors.WrapErr(err, "create repository directory")
	}
	lock, err := os.OpenFile(path.Join(repoPath, ".publication.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, errors.WrapErr(err, "open publication lock")
	}
	fd := int(lock.Fd()) //nolint:gosec // Unix file descriptors fit in int on supported targets.
	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		_ = lock.Close()
		return nil, errors.WrapErr(err, "lock publication")
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			_ = syscall.Flock(fd, syscall.LOCK_UN)
			_ = lock.Close()
		})
	}, nil
}

func writeAtomicFile(repoPath, dstFilePath string, file stream.SeekFile) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return errors.WrapErr(err, "seek source file")
	}
	tmp, err := os.CreateTemp(repoPath, ".object-*")
	if err != nil {
		return errors.WrapErr(err, "create object temp file")
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(0o644); err != nil { //nolint:gosec // published pacman repo file is world-readable by design
		_ = tmp.Close()
		return errors.WrapErr(err, "set object mode")
	}
	if _, err := io.Copy(tmp, file); err != nil {
		_ = tmp.Close()
		return errors.WrapErr(err, "copy object")
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return errors.WrapErr(err, "sync object")
	}
	if err := tmp.Close(); err != nil {
		return errors.WrapErr(err, "close object")
	}
	if err := os.Rename(tmpPath, dstFilePath); err != nil {
		return errors.WrapErr(err, "replace object")
	}
	dir, err := os.Open(repoPath)
	if err != nil {
		return errors.WrapErr(err, "open object directory")
	}
	defer func() { _ = dir.Close() }()
	return errors.WrapErr(dir.Sync(), "sync object directory")
}

func (l *LocalStore) StoreFileWithSignedURL(repo string, arch string, name string) (string, error) {
	return "", nil
}

// StoreFileWithSignedPutURL is unsupported on localfs: there is no object store to
// PUT to, so the client falls back to the multipart upload.
func (l *LocalStore) StoreFileWithSignedPutURL(repo string, arch string, name string) (string, error) {
	return "", blob.ErrPresignUnsupported
}

// FetchFileWithETag returns the file and its SHA-256 version.
func (l *LocalStore) FetchFileWithETag(repo, arch, file string) (stream.File, string, error) {
	f, err := l.FetchFile(repo, arch, file)
	if err != nil {
		return nil, "", err
	}
	seek, ok := f.(stream.SeekFile)
	if !ok {
		_ = f.Close()
		return nil, "", errors.New("local object stream is not seekable")
	}
	h := sha256.New()
	if _, err := io.Copy(h, seek); err != nil {
		_ = f.Close()
		return nil, "", errors.WrapErr(err, "hash local object")
	}
	if _, err := seek.Seek(0, io.SeekStart); err != nil {
		_ = f.Close()
		return nil, "", errors.WrapErr(err, "rewind local object")
	}
	return f, fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}

// FetchFileWithMeta returns the file with its on-disk modification time (the ETag
// stays empty; localfs has no object versioning). FetchFile already validated the
// path, so the re-stat is safe; a stat error just leaves LastModified zero.
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

// StoreFileIfMatch performs a locked conditional write.
func (l *LocalStore) StoreFileIfMatch(repo, arch string, file stream.SeekFile, etag string) error {
	name := path.Base(file.FileName())
	repoPath, dstFilePath, err := l.prepareObjectPath(repo, arch, name)
	if err != nil {
		return err
	}
	return withObjectLock(repoPath, name, func() error {
		current, hashErr := localFileETag(dstFilePath)
		switch {
		case errors.Is(hashErr, os.ErrNotExist):
			if etag != "" {
				return blob.ErrPreconditionFailed
			}
		case hashErr != nil:
			return hashErr
		case etag == "" || current != etag:
			return blob.ErrPreconditionFailed
		}
		return writeAtomicFile(repoPath, dstFilePath, file)
	})
}

func localFileETag(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", errors.WrapErr(err, "hash current object")
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}

func (l *LocalStore) FetchFile(repo string, arch string, file string) (stream.File, error) {
	repoDir, err := l.getRepoDir(repo)
	if err != nil {
		return nil, err
	}

	if err := blob.ValidatePathComponent(arch); err != nil {
		return nil, err
	}
	if err := blob.ValidatePathComponent(file); err != nil {
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

	if err := blob.ValidatePathComponent(arch); err != nil {
		return err
	}
	if err := blob.ValidatePathComponent(file); err != nil {
		return err
	}

	pkgPath := path.Join(repoDir, arch, file)
	slog.Info("remove pkg file", "file", pkgPath)
	if err := os.Remove(pkgPath); err != nil {
		slog.Warn("remove pkg file err", "err", err)
		return errors.WrapErr(err, "failed to remove pkg file")
	}

	return nil
}

// DeleteFileIfUnchanged removes an old object if its version is unchanged.
func (l *LocalStore) DeleteFileIfUnchanged(repo, arch string, expected blob.FileInfo, cutoff time.Time) (bool, error) {
	if expected.Name == "" || expected.LastModified.IsZero() {
		return false, nil
	}
	repoPath, objectPath, err := l.prepareObjectPath(repo, arch, expected.Name)
	if err != nil {
		return false, err
	}
	deleted := false
	err = withObjectLock(repoPath, expected.Name, func() error {
		fi, statErr := os.Stat(objectPath)
		if errors.Is(statErr, os.ErrNotExist) {
			return nil
		}
		if statErr != nil {
			return statErr
		}
		if !fi.ModTime().Equal(expected.LastModified) || fi.ModTime().After(cutoff) {
			return nil
		}
		if expected.Version != "" {
			version, versionErr := localFileETag(objectPath)
			if versionErr != nil {
				return versionErr
			}
			if version != expected.Version {
				return nil
			}
		}
		if err := os.Remove(objectPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		dir, err := os.Open(repoPath)
		if err != nil {
			return err
		}
		syncErr := dir.Sync()
		closeErr := dir.Close()
		if syncErr != nil {
			return syncErr
		}
		if closeErr != nil {
			return closeErr
		}
		deleted = true
		return nil
	})
	return deleted, err
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
		// A missing (repo, arch) dir has no files: match the s3 backend, which lists
		// an absent prefix as empty rather than erroring.
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, nil
		}
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

// FilesWithMeta lists (repo, arch) files with each file's modification time, for
// the orphan reconcile. An entry whose stat fails is returned with a zero time
// rather than dropped, so it is never GC'd on a transient stat error.
func (l *LocalStore) FilesWithMeta(repo string, arch string) ([]blob.FileInfo, error) {
	repoDir, err := l.getRepoDir(repo)
	if err != nil {
		return nil, err
	}
	repoPath := path.Join(repoDir, arch)

	entries, err := os.ReadDir(repoPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []blob.FileInfo{}, nil
		}
		return nil, err
	}
	infos := []blob.FileInfo{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		objectPath := path.Join(repoPath, entry.Name())
		info := blob.FileInfo{Name: entry.Name()}
		if fi, serr := os.Stat(objectPath); serr == nil {
			info.LastModified = fi.ModTime()
			if version, verr := localFileETag(objectPath); verr == nil {
				info.Version = version
			}
		}
		infos = append(infos, info)
	}
	return infos, nil
}
