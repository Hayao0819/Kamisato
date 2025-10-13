// Package型とその操作
package pkg

import (
	"archive/tar"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
	"github.com/Hayao0819/nahi/futils"
)

var ErrSRCINFONotFound = fmt.Errorf(".SRCINFO not found")

type Package struct {
	srcdir  string
	bin     string
	srcinfo *raiou.SRCINFO
	pkginfo *raiou.PKGINFO
	// desc     *raiou.DESC
	onmemory bool
}

func (p *Package) MustPKGINFO() *raiou.PKGINFO {
	info, err := p.PKGINFO()
	if err != nil {
		panic("failed to get pkginfo: " + err.Error())
	}
	return info
}

func (p *Package) PKGINFO() (*raiou.PKGINFO, error) {
	if p.pkginfo == nil {
		return nil, fmt.Errorf("pkginfo not found")
	}
	return p.pkginfo, nil
}

func (p *Package) SRCINFO() (*raiou.SRCINFO, error) {
	if p.srcinfo == nil {
		return nil, fmt.Errorf("srcinfo not found")
	}
	return p.srcinfo, nil
}

func PkgFromSrc(dir string) (*Package, error) {
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
	pkg.onmemory = false

	return pkg, nil
}

func PkgFromBinFile(bin string) (*Package, error) {
	file, err := os.Open(bin)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	pkg, err := PkgFromBin(bin, file)
	if err != nil {
		return nil, fmt.Errorf("failed to get pkg from bin: %w", err)
	}
	pkg.bin = bin
	pkg.onmemory = false
	return pkg, nil
}

// PkgFromBinは、指定されたパスからパッケージを取得します。
func PkgFromBin(bin string, r io.Reader) (*Package, error) {

	// zstdデコーダーを作成
	decoder, _, err := utils.DetectCompression(r)
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd decoder: %w", err)
	}
	defer decoder.Close()

	// tarリーダーを作成
	tarReader := tar.NewReader(decoder)

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
	pkg.bin = bin
	pkg.pkginfo = info
	pkg.onmemory = true

	return pkg, nil
}
