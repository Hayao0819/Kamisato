package utils

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
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

// CopyDir recursively copies a directory tree from src to dst.
// dst must not exist (it will be created).
func CopyDir(src, dst string) error {
	// srcの情報取得
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source dir: %w", err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not a directory")
	}

	// dstディレクトリ作成
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}

	// WalkFunc
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// srcからの相対パス
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		info, err := d.Info()
		if err != nil {
			return err
		}

		switch {
		case d.IsDir():
			// サブディレクトリ作成
			if err := os.MkdirAll(dstPath, info.Mode()); err != nil {
				return err
			}
		case (info.Mode() & os.ModeSymlink) != 0:
			// シンボリックリンク対応
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.Symlink(linkTarget, dstPath); err != nil {
				return err
			}
		default:
			// 通常ファイルコピー
			if err := copyFile(path, dstPath, info.Mode()); err != nil {
				return err
			}
		}
		return nil
	})
}

// ファイルコピー関数
func copyFile(srcFile, dstFile string, mode fs.FileMode) error {
	src, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(dstFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

func MoveFile(org string, dst string) error {
	var orgabs, dstabs string
	var err error

	orgabs, err = filepath.Abs(org)
	if err != nil {
		return err
	}

	dstabs, err = filepath.Abs(dst)
	if err != nil {
		return err
	}

	// If the file is already in the destination, do nothing
	if path.Dir(orgabs) == path.Dir(dstabs) && path.Base(orgabs) == path.Base(dstabs) {
		return nil
	}
	// If the file directory is the same, just rename it
	if path.Dir(orgabs) == path.Dir(dstabs) {
		return os.Rename(orgabs, dstabs)
	}

	// If the file is not in the same directory, copy it and delete the original

	// Open the original file
	orgfile, err := os.Open(orgabs)
	if err != nil {
		return err
	}
	defer orgfile.Close()

	// Move the file to the directory
	if dststat, err := os.Stat(dstabs); err == nil && dststat.IsDir() || path.Base(orgabs) == path.Base(dstabs) {
		dstabs = path.Join(dstabs, path.Base(orgabs))
	}

	// Create the parent directory
	if err := os.MkdirAll(path.Dir(dstabs), 0755); err != nil {
		return err
	}

	// Create the destination file
	dstfile, err := os.Create(dstabs)
	if err != nil {
		return err
	}
	defer dstfile.Close()

	// Copy the file
	_, err = io.Copy(dstfile, orgfile)
	orgfile.Close()
	if err != nil {
		return err
	}

	// Delete the original file
	err = os.Remove(orgabs)
	if err != nil {
		return err
	}

	return nil
}
