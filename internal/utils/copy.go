package utils

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

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
