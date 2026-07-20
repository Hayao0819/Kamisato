package safefile

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/gofrs/flock"
)

const defaultLockPollInterval = 50 * time.Millisecond

// Lock waits until it owns an exclusive advisory lock on path.
//
// Lock files must remain at stable paths after Unlock. Removing or atomically
// replacing one can let processes lock different inodes at the same time.
func Lock(path string, perm fs.FileMode) (*flock.Flock, error) {
	lock, err := newLock(path, perm)
	if err != nil {
		return nil, err
	}
	if err := lock.Lock(); err != nil {
		return nil, fmt.Errorf("safefile: lock %q: %w", path, err)
	}
	return finishLock(lock, path, perm)
}

// LockContext polls until it owns an exclusive advisory lock or ctx is done.
func LockContext(
	ctx context.Context,
	path string,
	perm fs.FileMode,
	pollInterval time.Duration,
) (*flock.Flock, error) {
	if pollInterval <= 0 {
		pollInterval = defaultLockPollInterval
	}
	lock, err := newLock(path, perm)
	if err != nil {
		return nil, err
	}
	acquired, err := lock.TryLockContext(ctx, pollInterval)
	if err != nil {
		return nil, fmt.Errorf("safefile: lock %q: %w", path, err)
	}
	if !acquired {
		return nil, fmt.Errorf("safefile: lock %q was not acquired", path)
	}
	return finishLock(lock, path, perm)
}

func newLock(path string, perm fs.FileMode) (*flock.Flock, error) {
	if path == "" {
		return nil, errors.New("safefile: empty lock path")
	}
	if perm != perm.Perm() {
		return nil, fmt.Errorf("safefile: invalid permission bits %#o", perm)
	}
	return flock.New(path, flock.SetPermissions(perm)), nil
}

func finishLock(lock *flock.Flock, path string, perm fs.FileMode) (*flock.Flock, error) {
	// flock applies perm when creating the file. Chmod also normalizes a lock
	// left by an older version or an operator with broader permissions.
	if err := os.Chmod(path, perm); err != nil {
		return nil, errors.Join(
			fmt.Errorf("safefile: set permissions on lock %q: %w", path, err),
			lock.Unlock(),
		)
	}
	return lock, nil
}
