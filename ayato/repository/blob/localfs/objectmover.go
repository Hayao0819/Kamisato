package localfs

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Hayao0819/nahi/futils"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/safefile"
)

var _ blob.ObjectMover = (*LocalStore)(nil)

func (l *LocalStore) root() (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return futils.ResolvePath(pwd, l.repoDir), nil
}

// rawPath maps a key onto a path under the repo root, rejecting any segment that
// could escape it.
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
	return errors.WrapErr(replaceObject(dst, in), "publish destination object")
}

// ListObjects walks prefix as a directory subtree; an absent prefix yields no keys.
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

func (l *LocalStore) DeleteObject(objKey string) error {
	p, err := l.rawPath(objKey)
	if err != nil {
		return err
	}
	if err := safefile.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.WrapErr(err, "remove object")
	}
	return nil
}
