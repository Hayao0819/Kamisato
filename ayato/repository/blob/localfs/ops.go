package localfs

import (
	"log/slog"
	"os"
	"path"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/atomicfile"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

func (l *LocalStore) StoreFile(repo, arch string, file stream.SeekFile) error {
	name := path.Base(file.FileName())
	repoPath, objectPath, err := l.prepareObjectPath(repo, arch, name)
	if err != nil {
		return err
	}
	return withObjectLock(repoPath, name, func() error {
		return writeAtomicFile(objectPath, file)
	})
}

func (l *LocalStore) StoreFileWithSignedURL(
	repo, arch, name string,
) (string, error) {
	return "", nil
}

// StoreFileIfMatch performs a locked conditional write against the local
// SHA-256 ETag.
func (l *LocalStore) StoreFileIfMatch(
	repo, arch string,
	file stream.SeekFile,
	etag string,
) error {
	name := path.Base(file.FileName())
	repoPath, objectPath, err := l.prepareObjectPath(repo, arch, name)
	if err != nil {
		return err
	}
	return withObjectLock(repoPath, name, func() error {
		current, hashErr := localFileETag(objectPath)
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
		return writeAtomicFile(objectPath, file)
	})
}

func (l *LocalStore) DeleteFile(repo, arch, name string) error {
	repoPath, objectPath, err := l.objectPath(repo, arch, name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(repoPath); err != nil {
		return errors.WrapErr(err, "failed to open package architecture directory")
	}
	slog.Info("remove pkg file", "file", objectPath)
	return withObjectLock(repoPath, name, func() error {
		if err := atomicfile.Remove(objectPath); err != nil {
			slog.Warn("remove pkg file err", "err", err)
			return errors.WrapErr(err, "failed to remove pkg file")
		}
		return nil
	})
}

// DeleteFileIfUnchanged removes an old object only while both its timestamp and
// optional content version still match the caller's listing snapshot.
func (l *LocalStore) DeleteFileIfUnchanged(
	repo, arch string,
	expected blob.FileInfo,
	cutoff time.Time,
) (bool, error) {
	if expected.Name == "" || expected.LastModified.IsZero() {
		return false, nil
	}
	repoPath, objectPath, err := l.prepareObjectPath(repo, arch, expected.Name)
	if err != nil {
		return false, err
	}
	deleted := false
	err = withObjectLock(repoPath, expected.Name, func() error {
		info, err := os.Stat(objectPath)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if !info.ModTime().Equal(expected.LastModified) || info.ModTime().After(cutoff) {
			return nil
		}
		if expected.Version != "" {
			version, err := localFileETag(objectPath)
			if err != nil {
				return err
			}
			if version != expected.Version {
				return nil
			}
		}
		if err := atomicfile.Remove(objectPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		deleted = true
		return nil
	})
	return deleted, err
}
