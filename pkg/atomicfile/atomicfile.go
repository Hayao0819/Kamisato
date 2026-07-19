// Package atomicfile durably replaces individual files.
//
// A replacement is written to a uniquely named sibling, flushed, and renamed
// over the destination. The parent directory is then flushed so a successful
// call means both the file contents and the rename reached durable storage.
package atomicfile

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Replace writes a complete new version of path.
//
// The destination's parent directory must already exist. write must not close
// the supplied writer. Until write returns successfully and the temporary file
// has been flushed, the old destination remains untouched. As with os.Rename,
// a final-path symbolic link is replaced rather than followed.
func Replace(path string, perm fs.FileMode, write func(io.Writer) error) error {
	if path == "" {
		return errors.New("atomicfile: empty destination path")
	}
	if perm != perm.Perm() {
		return fmt.Errorf("atomicfile: invalid permission bits %#o", perm)
	}
	if write == nil {
		return errors.New("atomicfile: nil write function")
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".atomic-*")
	if err != nil {
		return fmt.Errorf("atomicfile: create temporary file for %q: %w", path, err)
	}
	tmpPath := tmp.Name()
	closed := false
	defer func() {
		if !closed {
			_ = tmp.Close()
		}
		_ = os.Remove(tmpPath)
	}()

	if err := write(tmp); err != nil {
		return fmt.Errorf("atomicfile: write temporary file for %q: %w", path, err)
	}
	if err := tmp.Chmod(perm); err != nil {
		return fmt.Errorf("atomicfile: set permissions on temporary file for %q: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("atomicfile: sync temporary file for %q: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		closed = true
		return fmt.Errorf("atomicfile: close temporary file for %q: %w", path, err)
	}
	closed = true
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("atomicfile: replace %q: %w", path, err)
	}
	if err := SyncDirectory(dir); err != nil {
		return fmt.Errorf("atomicfile: commit replacement of %q: %w", path, err)
	}
	return nil
}

// WriteFile durably replaces path with data.
func WriteFile(path string, data []byte, perm fs.FileMode) error {
	return Replace(path, perm, func(w io.Writer) error {
		_, err := io.Copy(w, bytes.NewReader(data))
		return err
	})
}

// Remove deletes path and flushes its parent directory.
func Remove(path string) error {
	if path == "" {
		return errors.New("atomicfile: empty path")
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("atomicfile: remove %q: %w", path, err)
	}
	if err := SyncDirectory(filepath.Dir(path)); err != nil {
		return fmt.Errorf("atomicfile: commit removal of %q: %w", path, err)
	}
	return nil
}

// SyncDirectory flushes directory-entry changes made beneath path.
func SyncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open directory %q: %w", path, err)
	}
	if err := dir.Sync(); err != nil {
		_ = dir.Close()
		return fmt.Errorf("sync directory %q: %w", path, err)
	}
	if err := dir.Close(); err != nil {
		return fmt.Errorf("close directory %q: %w", path, err)
	}
	return nil
}
