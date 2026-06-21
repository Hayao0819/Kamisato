// Remote repository (a set of binary packages from a .db).
package repo

import (
	"archive/tar"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	utils "github.com/Hayao0819/Kamisato/internal/utils"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

type RemoteRepo struct {
	Name   string
	Server string
	Pkgs   []*pkg.BinaryPackage
}

func (r *RemoteRepo) PkgByPkgName(pkgname string) *pkg.BinaryPackage {
	for _, p := range r.Pkgs {
		if p.Name() == pkgname {
			return p
		}
	}
	return nil
}

func (r *RemoteRepo) PkgByPkgBase(pkgbase string) *pkg.BinaryPackage {
	for _, p := range r.Pkgs {
		if p.Base() == pkgbase {
			return p
		}
	}
	return nil
}

// RepoFromURL fetches the remote repository from the given URL.
func RepoFromURL(server string, name string) (*RemoteRepo, error) {
	dburl, err := url.JoinPath(server, name+".db")
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(dburl)
	if err != nil {
		return nil, fmt.Errorf("failed to download database: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status while downloading: %s", resp.Status)
	}

	r, err := RemoteRepoFromDB(name, resp.Body)
	if r != nil {
		r.Server = server
	}
	return r, err
}

func RepoFromDBFile(name string, dbfile string) (*RemoteRepo, error) {
	db, err := os.Open(dbfile)
	if err != nil {
		return nil, fmt.Errorf("failed to open database file: %w", err)
	}
	defer db.Close()

	return RemoteRepoFromDB(name, db)
}

// RemoteRepoFromDB parses a repository .db stream (a compressed tar of
// "<pkg>-<ver>/desc" entries) into a RemoteRepo.
func RemoteRepoFromDB(name string, db io.Reader) (*RemoteRepo, error) {
	gzr, _, err := utils.DetectCompression(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create decompressor: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	pkgs := make([]*pkg.BinaryPackage, 0)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read db tar: %w", err)
		}
		if hdr.Typeflag == tar.TypeDir || !strings.HasSuffix(hdr.Name, "/desc") {
			continue
		}

		var buf strings.Builder
		if _, err := io.Copy(&buf, tr); err != nil { //nolint:gosec // db is operator-controlled
			return nil, fmt.Errorf("failed to read %s: %w", hdr.Name, err)
		}
		desc, err := raiou.ParseDescString(buf.String())
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", hdr.Name, err)
		}
		info, err := desc.ToPKGINFO()
		if err != nil {
			return nil, fmt.Errorf("failed to convert %s: %w", hdr.Name, err)
		}
		pkgs = append(pkgs, pkg.NewBinaryPackage(desc.FileName, info))
	}
	return &RemoteRepo{Name: name, Pkgs: pkgs}, nil
}
