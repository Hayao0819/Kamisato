// リモートリポジトリ（.db 由来のバイナリパッケージ集合）
package repo

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	utils "github.com/Hayao0819/Kamisato/internal/utils"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
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

// RemoteRepoFromDB parses a repository .db stream into a RemoteRepo.
// TODO: tar 展開とパッケージ読込処理は未実装。
func RemoteRepoFromDB(name string, db io.Reader) (*RemoteRepo, error) {
	gzr, _, err := utils.DetectCompression(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create decompressor: %w", err)
	}
	defer gzr.Close()

	// tr := tar.NewReader(gzr)
	pkgs := make([]*pkg.BinaryPackage, 0)
	// ...（省略: tar展開とパッケージ読込処理）...
	return &RemoteRepo{Name: name, Pkgs: pkgs}, nil
}
