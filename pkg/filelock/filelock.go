// Package filelock provides process-shared advisory file locks.
//
// Lock paths must be stable: callers should keep the empty lock file in place
// after releasing it. Removing or atomically replacing a lock file can let two
// processes lock different inodes and enter the protected section together.
package filelock

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"
	"syscall"
	"time"
)

// Guard owns an exclusive advisory lock. Release is safe to call more than
// once, although callers should normally defer the first call.
type Guard struct {
	file    *os.File
	once    sync.Once
	release error
}

// Acquire waits until it owns an exclusive lock on path.
func Acquire(path string, perm fs.FileMode) (*Guard, error) {
	guard, err := open(path, perm)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(guard.fd(), syscall.LOCK_EX); err != nil {
		_ = guard.file.Close()
		return nil, fmt.Errorf("filelock: lock %q: %w", path, err)
	}
	return guard, nil
}

// AcquireContext polls for an exclusive lock until it succeeds or ctx is done.
// A non-positive pollInterval uses 50 milliseconds.
func AcquireContext(ctx context.Context, path string, perm fs.FileMode, pollInterval time.Duration) (*Guard, error) {
	if pollInterval <= 0 {
		pollInterval = 50 * time.Millisecond
	}
	guard, err := open(path, perm)
	if err != nil {
		return nil, err
	}
	for {
		if err := ctx.Err(); err != nil {
			_ = guard.file.Close()
			return nil, err
		}
		err = syscall.Flock(guard.fd(), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return guard, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			_ = guard.file.Close()
			return nil, fmt.Errorf("filelock: lock %q: %w", path, err)
		}

		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			_ = guard.file.Close()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

// TryAcquire attempts an exclusive lock without waiting. A contended lock
// returns (nil, false, nil).
func TryAcquire(path string, perm fs.FileMode) (*Guard, bool, error) {
	guard, err := open(path, perm)
	if err != nil {
		return nil, false, err
	}
	err = syscall.Flock(guard.fd(), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		return guard, true, nil
	}
	_ = guard.file.Close()
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return nil, false, nil
	}
	return nil, false, fmt.Errorf("filelock: lock %q: %w", path, err)
}

func open(path string, perm fs.FileMode) (*Guard, error) {
	if path == "" {
		return nil, errors.New("filelock: empty lock path")
	}
	if perm != perm.Perm() {
		return nil, fmt.Errorf("filelock: invalid permission bits %#o", perm)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, perm)
	if err != nil {
		return nil, fmt.Errorf("filelock: open %q: %w", path, err)
	}
	if err := file.Chmod(perm); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("filelock: set permissions on %q: %w", path, err)
	}
	return &Guard{file: file}, nil
}

func (g *Guard) fd() int {
	return int(g.file.Fd()) //nolint:gosec // Unix file descriptors fit in int on every supported target.
}

// Release unlocks and closes the lock file.
func (g *Guard) Release() error {
	if g == nil {
		return nil
	}
	g.once.Do(func() {
		unlockErr := syscall.Flock(g.fd(), syscall.LOCK_UN)
		closeErr := g.file.Close()
		g.release = errors.Join(unlockErr, closeErr)
	})
	return g.release
}
