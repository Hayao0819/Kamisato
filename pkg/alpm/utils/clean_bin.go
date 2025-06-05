package utils

import (
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/pkg/alpm/pkg"
	remote "github.com/Hayao0819/Kamisato/pkg/alpm/remoterepo"
	"github.com/Hayao0819/nahi/exutils"
	"github.com/Hayao0819/nahi/flist"
	"github.com/Hayao0819/nahi/futils"
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

func GetCleanPkgBinary(names ...string) ([]string, error) {
	tmp, err := os.MkdirTemp("", "kamisato-pkg-dl-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	// defer os.RemoveAll(tmp)

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

	repodbfiles, err := flist.Get(path.Join(dbpath, "sync"), flist.WithFileOnly(), flist.WithExactDepth(1), flist.WithExtOnly(".db"))
	if err != nil {
		return nil, fmt.Errorf("failed to list db files: %w", err)
	}
	if len(*repodbfiles) == 0 {
		return nil, fmt.Errorf("no db files found in %s", dbpath)
	}

	remoterepos := []*remote.RemoteRepo{}
	for _, dbfile := range *repodbfiles {
		rr, err := remote.GetRepoFromDBFile(futils.BaseWithoutExt(dbfile), dbfile)
		if err != nil {
			slog.Error("failed to get repo from db file", "file", dbfile, "error", err)
			continue
		}
		remoterepos = append(remoterepos, rr)
	}

	if len(remoterepos) == 0 {
		return nil, fmt.Errorf("no remote repositories found in db files")
	}

	pkgs := make([]*pkg.Package, 0)
	for _, rr := range remoterepos {
		for _, pkg := range rr.Pkgs {
			pkgs = append(pkgs, pkg)
		}
	}

	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages found in remote repositories")
	}

	rt := make(([]string), len(pkgs))
	for _, p := range pkgs {
		for _, n := range names {
			slog.Debug("Extract filename for package", "name", n)
			d := p.MustDesc()
			rt = append(rt, d.FileName)
			// TODO: Support desc for sync
		}
	}

	return rt, nil
}
