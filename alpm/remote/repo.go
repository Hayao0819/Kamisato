package remote

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/Hayao0819/Kamisato/alpm/pkg"
)

type RemoteRepo struct {
	Name string
	// Url  string
	Pkgs []*pkg.Package
}

// GetRepoFromURL fetches the remote repository from the given URL.
// Example URL: https://cdnmirror.com/archlinux/core/os/x86_64
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

	return getRepo(name, resp.Body)
}

func GetRepoFromDBFile(name string, dbfile string) (*RemoteRepo, error) {
	db, err := os.Open(dbfile)
	if err != nil {
		return nil, fmt.Errorf("failed to open database file: %w", err)
	}
	defer db.Close()

	return getRepo(name, db)
}

func getRepo(name string, db io.Reader) (*RemoteRepo, error) {
	// gzip リーダーを作成
	gzr, err := gzip.NewReader(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	// tar リーダーを作成
	tr := tar.NewReader(gzr)

	pkgs := make([]*pkg.Package, 0)

	// 各エントリを読み込む
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // 終了
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar entry: %w", err)
		}

		// ディレクトリはスキップ
		if hdr.Typeflag == tar.TypeDir {
			continue
		}

		// fmt.Printf("File: %s\n", hdr.Name)

		// ファイルの内容を読み込んで出力
		p, err := pkg.GetPkgFromDesc(tr)
		if err != nil {
			return nil, fmt.Errorf("failed to get package from description: %w", err)
		}

		if p == nil {
			continue
		}
		pkgs = append(pkgs, p)
	}

	return &RemoteRepo{
		Name: "remote",
		Pkgs: pkgs,
	}, nil
}
