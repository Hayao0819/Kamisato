package repo

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/Morganamilo/go-srcinfo"
	"github.com/klauspost/compress/zstd"
)

type Info srcinfo.Srcinfo

type Package struct {
	path string
	info *Info
}

func (p *Package) Info() *Info {
	return p.info
}

func GetPkgFromSrc(pkgbuild string) (*Package, error) {
	info, err := srcinfo.ParseFile(path.Join(path.Dir(pkgbuild), ".SRCINFO"))
	if err != nil {
		return nil, err
	}

	pkg := new(Package)
	pkg.path = pkgbuild
	pkg.info = (*Info)(info)

	return pkg, nil
}

func GetPkgFromBin(name string) (*Package, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// zstdデコーダーを作成
	zstdDecoder, err := zstd.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd decoder: %w", err)
	}
	defer zstdDecoder.Close()

	// tarリーダーを作成
	tarReader := tar.NewReader(zstdDecoder)

	// .BININFOファイルを探す
	var bininfoData string
	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("failed to read tar header: %w", err)
		}

		if header.Name == ".BININFO" {
			// .BININFOファイルの内容を読み取る
			buf := new(strings.Builder)
			if _, err := io.Copy(buf, tarReader); err != nil {
				return nil, fmt.Errorf("failed to read .BININFO: %w", err)
			}
			bininfoData = buf.String()
			break
		}
	}

	if bininfoData == "" {
		return nil, fmt.Errorf(".BININFO not found in archive")
	}

	// srcinfoを解析
	info, err := srcinfo.Parse(bininfoData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse srcinfo: %w", err)
	}

	pkg := new(Package)
	pkg.path = name
	pkg.info = (*Info)(info)

	return pkg, nil
}
