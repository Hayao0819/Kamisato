// リモートリポジトリ取得
package remote

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	utils "github.com/Hayao0819/Kamisato/internal"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/package"
)

// GetRepoFromURL fetches the remote repository from the given URL.
func GetRepoFromURL(server string, name string) (*RemoteRepo, error) {
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

	return GetRepo(name, resp.Body)
}

func GetRepoFromDBFile(name string, dbfile string) (*RemoteRepo, error) {
	db, err := os.Open(dbfile)
	if err != nil {
		return nil, fmt.Errorf("failed to open database file: %w", err)
	}
	defer db.Close()

	return GetRepo(name, db)
}

func GetRepo(name string, db io.Reader) (*RemoteRepo, error) {
	gzr, _, err := utils.DetectCompression(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	// tr := tar.NewReader(gzr)
	pkgs := make([]*pkg.Package, 0)
	// ...（省略: tar展開とパッケージ読込処理）...
	return &RemoteRepo{Name: name, Pkgs: pkgs}, nil
}
