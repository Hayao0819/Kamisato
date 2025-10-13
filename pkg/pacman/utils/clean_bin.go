// パッケージバイナリの一時取得・クリーンアップ
package utils

import (
	"fmt"
	"os"
	"path"

	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/package"
	"github.com/Hayao0819/nahi/exutils"
	"github.com/Hayao0819/nahi/flist"
)

type CleanPkgBinary struct {
	path string
	dir  string
}

func (c *CleanPkgBinary) Close() error {
	if c.path == "" {
		return nil
	}
	if err := os.Remove(c.path); err != nil {
		return fmt.Errorf("failed to remove package binary: %w", err)
	}
	if err := os.RemoveAll(c.dir); err != nil {
		return fmt.Errorf("failed to remove temp directory: %w", err)
	}
	c.path = ""
	c.dir = ""
	return nil
}

// TODO: 実装
func GetCleanPkgBinary(names ...string) ([]string, error) {

	if len(names) == 0 {
		return nil, nil
	}

	pkgs := make([]*pkg.Package, 0)
	_ = pkgs // 型利用のため一時的に追加
	tmp, err := os.MkdirTemp("", "kamisato-pkg-dl-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	dbpath := path.Join(tmp, "db")
	if err := os.MkdirAll(dbpath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}
	cachepath := path.Join(tmp, "cache")
	if err := os.MkdirAll(cachepath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	args := []string{"pacman", "--sync", "--refresh", "--noconfirm", "--downloadonly", "--cachedir", cachepath, "--dbpath", dbpath, "--log", "/dev/null"}
	args = append(args, names...)
	c := exutils.CommandWithStdio("fakeroot", args...)
	if err := c.Run(); err != nil {
		return nil, fmt.Errorf("failed to download package: %w", err)
	}

	_, err = flist.Get(path.Join(dbpath, "sync"), flist.WithFileOnly(), flist.WithExactDepth(1), flist.WithExtOnly(".db"))
	if err != nil {
		return nil, fmt.Errorf("failed to get repo db files: %w", err)
	}
	// ...（省略: パッケージファイル取得処理）...
	return nil, nil
}
