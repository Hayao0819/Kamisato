package pkg

import (
	"archive/tar"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"strings"

	"github.com/Hayao0819/Kamisato/raiou"
	"github.com/Hayao0819/nahi/futils"
	"github.com/klauspost/compress/zstd"
)

var ErrSRCINFONotFound = fmt.Errorf(".SRCINFO not found")

type Package struct {
	srcdir  string
	bin     string
	srcinfo *raiou.SRCINFO
	pkginfo *raiou.PKGINFO
}

func (p *Package) SRCINFO() (*raiou.SRCINFO, error) {
	if p.srcinfo == nil {
		return nil, fmt.Errorf("srcinfo not found")
	}
	return p.srcinfo, nil
}

func (p *Package) PKGINFO() (*raiou.PKGINFO, error) {
	if p.pkginfo == nil {
		return nil, fmt.Errorf("pkginfo not found")
	}
	return p.pkginfo, nil
}

func (p *Package) MustSRCINFO() *raiou.SRCINFO {
	info, err := p.SRCINFO()
	if err != nil {
		panic("failed to get srcinfo: " + err.Error())
	}
	return info
}
func (p *Package) MustPKGINFO() *raiou.PKGINFO {
	info, err := p.PKGINFO()
	if err != nil {
		panic("failed to get pkginfo: " + err.Error())
	}
	return info
}

func GetPkgFromSrc(dir string) (*Package, error) {
	srcinfoFile := path.Join(dir, ".SRCINFO")
	if !futils.Exists(srcinfoFile) {
		return nil, ErrSRCINFONotFound
	}

	info, err := raiou.ParseSrcinfoFile(srcinfoFile)
	if err != nil {
		return nil, err
	}

	pkg := new(Package)
	pkg.srcdir = dir
	pkg.srcinfo = info

	return pkg, nil
}

// GetPkgFromBinは、指定されたパスからパッケージを取得します。
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

	var pkginfoData string
	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}
		slog.Info("tar header", "name", header.Name)

		if err != nil {
			return nil, fmt.Errorf("failed to read tar header: %w", err)
		}

		if header.Name == ".PKGINFO" {
			buf := new(strings.Builder)
			if _, err := io.Copy(buf, tarReader); err != nil {
				return nil, fmt.Errorf("failed to read .BININFO: %w", err)
			}
			pkginfoData = buf.String()
			break
		}
	}

	if pkginfoData == "" {
		return nil, fmt.Errorf(".PKGINFO not found in archive")
	}

	info, err := raiou.ParsePkginfoString(pkginfoData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse srcinfo: %w", err)
	}

	pkg := new(Package)
	pkg.bin = name
	pkg.pkginfo = info

	return pkg, nil
}

func GetPkgFromPKGINFO(bin string, pkginfoData string) (*Package, error) {
	info, err := raiou.ParsePkginfoString(pkginfoData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse srcinfo: %w", err)
	}

	pkg := new(Package)
	pkg.bin = bin
	pkg.pkginfo = info

	return pkg, nil
}
