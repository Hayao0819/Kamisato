package utils

import (
	"bytes"
	"io"
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
