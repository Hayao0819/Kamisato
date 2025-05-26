package remoterepo

import (
	"archive/tar"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"

	"github.com/Hayao0819/Kamisato/alpm/pkg"
	"github.com/Hayao0819/Kamisato/internal/utils"
)

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
	// gzip リーダーを作成
	gzr, _, err := utils.DetectCompression(db)
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
			slog.Debug("End of tar archive")
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
		slog.Debug("File", "name", hdr.Name)

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
		Name: name,
		Pkgs: pkgs,
	}, nil
}
