package localfs

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/nahi/futils"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

func (l *LocalStore) FetchFile(repo, arch, name string) (stream.File, error) {
	_, objectPath, err := l.objectPath(repo, arch, name)
	if err != nil {
		return nil, err
	}
	if !futils.Exists(objectPath) {
		return nil, blob.ErrNotFound
	}
	slog.Info("fetch pkg file", "file", objectPath)
	file, err := stream.OpenFileWithType(objectPath)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// FetchFileWithETag returns a seekable file and its content-addressed version.
func (l *LocalStore) FetchFileWithETag(
	repo, arch, name string,
) (stream.File, string, error) {
	file, err := l.FetchFile(repo, arch, name)
	if err != nil {
		return nil, "", err
	}
	seekable, ok := file.(stream.SeekFile)
	if !ok {
		_ = file.Close()
		return nil, "", errors.New("local object stream is not seekable")
	}
	etag, err := sha256ETag(seekable)
	if err != nil {
		_ = file.Close()
		return nil, "", errors.WrapErr(err, "hash local object")
	}
	if err := stream.Rewind(seekable); err != nil {
		_ = file.Close()
		return nil, "", errors.WrapErr(err, "rewind local object")
	}
	return file, etag, nil
}

func (l *LocalStore) FetchFileWithMeta(
	repo, arch, name string,
) (stream.File, blob.FileMeta, error) {
	file, err := l.FetchFile(repo, arch, name)
	if err != nil {
		return nil, blob.FileMeta{}, err
	}
	var meta blob.FileMeta
	if _, objectPath, pathErr := l.objectPath(repo, arch, name); pathErr == nil {
		if info, statErr := os.Stat(objectPath); statErr == nil {
			meta.LastModified = info.ModTime()
		}
	}
	return file, meta, nil
}

func localFileETag(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	etag, err := sha256ETag(file)
	return etag, errors.WrapErr(err, "hash current object")
}

func sha256ETag(reader io.Reader) (string, error) {
	hasher := sha256.New()
	if _, err := io.Copy(hasher, reader); err != nil {
		return "", err
	}
	return fmt.Sprintf("sha256:%x", hasher.Sum(nil)), nil
}
