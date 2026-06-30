package alpm

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/Hayao0819/nahi/exutils"
)

// CleanPkgBinary owns the temp dir holding the packages downloaded by
// GetCleanPkgBinary. Those files are injected during the build, so the caller
// must keep it alive until the build finishes and Close() it afterwards.
type CleanPkgBinary struct {
	dir string
}

func (c *CleanPkgBinary) Close() error {
	if c == nil || c.dir == "" {
		return nil
	}
	if err := os.RemoveAll(c.dir); err != nil {
		return fmt.Errorf("failed to remove temp directory: %w", err)
	}
	c.dir = ""
	return nil
}

// GetCleanPkgBinary downloads names from the pacman repos into a temp dir and
// returns the downloaded .pkg.tar.* file paths together with a handle to remove
// the temp dir once the build has consumed them.
func GetCleanPkgBinary(names ...string) ([]string, *CleanPkgBinary, error) {
	if len(names) == 0 {
		return nil, nil, nil
	}

	tmp, err := os.MkdirTemp("", "kamisato-pkg-dl-")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	cleanup := &CleanPkgBinary{dir: tmp}

	dbpath := path.Join(tmp, "db")
	if err := os.MkdirAll(dbpath, 0755); err != nil {
		_ = cleanup.Close()
		return nil, nil, fmt.Errorf("failed to create db directory: %w", err)
	}
	cachepath := path.Join(tmp, "cache")
	if err := os.MkdirAll(cachepath, 0755); err != nil {
		_ = cleanup.Close()
		return nil, nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// pacman 7's downloader sandbox (DownloadUser + Landlock) cannot initialize
	// under fakeroot (no real setuid) or in containers (Landlock unavailable), so
	// disable it; integrity still comes from pacman's signature verification.
	args := []string{"pacman", "--sync", "--refresh", "--noconfirm", "--downloadonly", "--disable-sandbox", "--cachedir", cachepath, "--dbpath", dbpath, "--log", "/dev/null"}
	args = append(args, names...)
	c := exutils.CommandWithStdio("fakeroot", args...)
	if err := c.Run(); err != nil {
		_ = cleanup.Close()
		return nil, nil, fmt.Errorf("failed to download package: %w", err)
	}

	entries, err := os.ReadDir(cachepath)
	if err != nil {
		_ = cleanup.Close()
		return nil, nil, fmt.Errorf("failed to list downloaded packages: %w", err)
	}
	var files []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.Contains(name, ".pkg.tar.") || strings.HasSuffix(name, ".sig") {
			continue
		}
		files = append(files, path.Join(cachepath, name))
	}
	return files, cleanup, nil
}
