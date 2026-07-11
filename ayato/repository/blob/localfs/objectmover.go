package localfs

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Hayao0819/nahi/futils"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

var _ blob.ObjectMover = (*LocalStore)(nil)

// root resolves the repo root the same way getRepoDir does, so raw-key operations
// map onto the same directory tree the servable API writes into.
func (l *LocalStore) root() (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return futils.ResolvePath(pwd, l.repoDir), nil
}

// rawPath maps a full object key onto an on-disk path under the repo root, rejecting
// any segment that could escape it.
func (l *LocalStore) rawPath(key string) (string, error) {
	root, err := l.root()
	if err != nil {
		return "", err
	}
	segs := strings.Split(key, "/")
	for _, seg := range segs {
		if err := blob.ValidatePathComponent(seg); err != nil {
			return "", err
		}
	}
	return filepath.Join(append([]string{root}, segs...)...), nil
}

// CopyObject copies srcKey to dstKey on disk (localfs has no server-side copy).
func (l *LocalStore) CopyObject(srcKey, dstKey string) error {
	src, err := l.rawPath(srcKey)
	if err != nil {
		return err
	}
	dst, err := l.rawPath(dstKey)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		if os.IsNotExist(err) {
			return blob.ErrNotFound
		}
		return errors.WrapErr(err, "open source object")
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { //nolint:gosec // published pacman repo dir is world-readable by design
		return errors.WrapErr(err, "mkdir destination")
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644) //nolint:gosec // published pacman repo file is world-readable by design
	if err != nil {
		return errors.WrapErr(err, "create destination object")
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return errors.WrapErr(err, "copy object")
	}
	return nil
}

// ListObjects returns every object key under prefix, treating it as a directory
// subtree; an absent prefix yields no keys.
func (l *LocalStore) ListObjects(prefix string) ([]string, error) {
	root, err := l.root()
	if err != nil {
		return nil, err
	}
	base := filepath.Join(root, filepath.FromSlash(prefix))
	var keys []string
	err = filepath.WalkDir(base, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		keys = append(keys, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return keys, nil
}

// DeleteObject removes objKey; a missing key is not an error.
func (l *LocalStore) DeleteObject(objKey string) error {
	p, err := l.rawPath(objKey)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return errors.WrapErr(err, "remove object")
	}
	return nil
}
