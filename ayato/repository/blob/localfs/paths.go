// Package localfs is a blob.Store backed by the local filesystem.
package localfs

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/pacman/reponame"
	"github.com/Hayao0819/Kamisato/pkg/safefile"

	"github.com/Hayao0819/nahi/futils"
	"github.com/samber/lo"

	"github.com/Hayao0819/Kamisato/ayato/platform"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
)

var _ blob.Store = (*LocalStore)(nil)

// LocalStore takes plain configuration values so the IO adapter does not depend
// on the application configuration package.
type LocalStore struct {
	repoDir   string
	repoNames []string
}

func New(repoDir string, repoNames []string) *LocalStore {
	return &LocalStore{repoDir: repoDir, repoNames: repoNames}
}

func (l *LocalStore) getRepoDir(name string) (string, error) {
	slog.Debug("get repo dir", "name", name)
	if err := reponame.Validate(name); err != nil {
		return "", err
	}
	pwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}
	if lo.Contains(l.repoNames, name) {
		return path.Join(futils.ResolvePath(pwd, l.repoDir), name), nil
	}
	return "", fmt.Errorf("%w: repo %s", blob.ErrNotFound, name)
}

func (l *LocalStore) archPath(repo, arch string) (string, error) {
	repoDir, err := l.getRepoDir(repo)
	if err != nil {
		return "", err
	}
	if err := blob.ValidatePathComponent(arch); err != nil {
		return "", err
	}
	return path.Join(repoDir, arch), nil
}

func (l *LocalStore) objectPath(
	repo, arch, name string,
) (repoPath, objectPath string, err error) {
	repoPath, err = l.archPath(repo, arch)
	if err != nil {
		return "", "", err
	}
	if err := blob.ValidatePathComponent(name); err != nil {
		return "", "", err
	}
	return repoPath, path.Join(repoPath, name), nil
}

func (l *LocalStore) prepareObjectPath(
	repo, arch, name string,
) (repoPath, objectPath string, err error) {
	repoPath, objectPath, err = l.objectPath(repo, arch, name)
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(repoPath, 0o755); err != nil { //nolint:gosec // published pacman repo
		return "", "", errors.WrapErr(err, fmt.Sprintf("mkdir %s err", repoPath))
	}
	return repoPath, objectPath, nil
}

func withObjectLock(repoPath, name string, operation func() error) error {
	lockDir := path.Join(repoPath, ".locks")
	if err := os.MkdirAll(lockDir, 0o755); err != nil { //nolint:gosec // lock dir has no secrets
		return errors.WrapErr(err, "create object lock directory")
	}
	lockName := fmt.Sprintf("%x.lock", sha256.Sum256([]byte(name)))
	lock, err := safefile.Lock(path.Join(lockDir, lockName), 0o600)
	if err != nil {
		return errors.WrapErr(err, "lock object")
	}
	defer func() { _ = lock.Unlock() }()
	return operation()
}

// LockPublication obtains the cross-process repository publication lock.
func (l *LocalStore) LockPublication(repo string) (func(), error) {
	repoPath, err := l.getRepoDir(repo)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(repoPath, 0o755); err != nil { //nolint:gosec // published repository
		return nil, errors.WrapErr(err, "create repository directory")
	}
	lock, err := safefile.Lock(path.Join(repoPath, ".publication.lock"), 0o600)
	if err != nil {
		return nil, errors.WrapErr(err, "lock publication")
	}
	return func() { _ = lock.Unlock() }, nil
}

func writeAtomicFile(dst string, file platform.SeekFile) error {
	if err := platform.Rewind(file); err != nil {
		return errors.WrapErr(err, "seek source file")
	}
	return errors.WrapErr(replaceObject(dst, file), "store object")
}

func replaceObject(dst string, source io.Reader) error {
	return safefile.Replace(dst, 0o644, func(out io.Writer) error { //nolint:gosec // published file
		_, err := io.Copy(out, source)
		return errors.WrapErr(err, "copy object")
	})
}
