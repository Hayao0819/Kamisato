package utils

import (
	"bytes"
	"io"
	"os"
	"path/filepath"

	"github.com/otiai10/copy"
)

type ReadSeekCloser struct {
	io.ReadSeeker
}

func (r ReadSeekCloser) Close() error {
	return nil
}

func BufferToReadSeekCloser(buf *bytes.Buffer) io.ReadSeekCloser {
	return ReadSeekCloser{
		ReadSeeker: bytes.NewReader(buf.Bytes()),
	}
}

func ResolvePath(baseDir, targetPath string) string {
	if filepath.IsAbs(targetPath) {
		return filepath.Clean(targetPath)
	}
	joined := filepath.Join(baseDir, targetPath)
	return filepath.Clean(joined)
}

// CopyDir recursively copies the src tree to dst, creating dst if needed.
func CopyDir(src, dst string) error {
	return copy.Copy(src, dst)
}

func MoveFile(org string, dst string) error {
	orgabs, err := filepath.Abs(org)
	if err != nil {
		return err
	}

	dstabs, err := filepath.Abs(dst)
	if err != nil {
		return err
	}

	// If dst is an existing directory, move into it keeping the original name.
	if dststat, err := os.Stat(dstabs); err == nil && dststat.IsDir() {
		dstabs = filepath.Join(dstabs, filepath.Base(orgabs))
	}
	if orgabs == dstabs {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dstabs), 0o755); err != nil {
		return err
	}

	// Rename is atomic within a single filesystem; fall back to a copy (which
	// preserves the source mode) across devices, dropping the source only once
	// the copy has succeeded.
	if err := os.Rename(orgabs, dstabs); err == nil {
		return nil
	}
	if err := copy.Copy(orgabs, dstabs); err != nil {
		return err
	}
	return os.Remove(orgabs)
}
