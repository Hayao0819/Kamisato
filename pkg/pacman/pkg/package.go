// Package pkg はビルド対象(SourcePackage)とビルド成果物(BinaryPackage)という
// 2 種類のパッケージをドメイン型として表します。メタ情報のパースは pkg/raiou に委ねます。
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

// SourcePackage は .SRCINFO / PKGBUILD ディレクトリ由来のビルド対象パッケージです。
type SourcePackage struct {
	dir  string
	info *raiou.SRCINFO
}

// OpenSourcePackage はディレクトリの .SRCINFO を読み取り SourcePackage を返します。
func OpenSourcePackage(dir string) (*SourcePackage, error) {
	srcinfoFile := path.Join(dir, ".SRCINFO")
	if !futils.Exists(srcinfoFile) {
		return nil, ErrSRCINFONotFound
	}

	info, err := raiou.ParseSrcinfoFile(srcinfoFile)
	if err != nil {
		return nil, err
	}

	return &SourcePackage{dir: dir, info: info}, nil
}

// SRCINFO は解析済みの .SRCINFO を返します。
func (p *SourcePackage) SRCINFO() *raiou.SRCINFO {
	return p.info
}

// Dir はソースディレクトリのパスを返します。
func (p *SourcePackage) Dir() string {
	return p.dir
}

// BinaryPackage は .pkg.tar / リポジトリ DB の desc 由来のビルド成果物パッケージです。
type BinaryPackage struct {
	path string
	info *raiou.PKGINFO
}

// OpenBinaryPackage はパッケージファイルを開き BinaryPackage を返します。
func OpenBinaryPackage(binPath string) (*BinaryPackage, error) {
	file, err := os.Open(binPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	return ReadBinaryPackage(binPath, file)
}

// ReadBinaryPackage は r から .PKGINFO を読み取り BinaryPackage を返します。
func ReadBinaryPackage(binPath string, r io.Reader) (*BinaryPackage, error) {
	decoder, _, err := utils.DetectCompression(r)
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}
	defer decoder.Close()

	tarReader := tar.NewReader(decoder)

	var pkginfoData string
	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar header: %w", err)
		}
		slog.Debug("tar header", "name", header.Name)

		if header.Name == ".PKGINFO" {
			buf := new(strings.Builder)
			if _, err := io.Copy(buf, tarReader); err != nil {
				return nil, fmt.Errorf("failed to read .PKGINFO: %w", err)
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
		return nil, fmt.Errorf("failed to parse pkginfo: %w", err)
	}

	return &BinaryPackage{path: binPath, info: info}, nil
}

// NewBinaryPackage wraps parsed metadata with its file path. For packages read
// from a repository .db, filePath is the desc %FILENAME% (a bare filename).
func NewBinaryPackage(filePath string, info *raiou.PKGINFO) *BinaryPackage {
	return &BinaryPackage{path: filePath, info: info}
}

// PKGINFO は解析済みの .PKGINFO を返します。
func (p *BinaryPackage) PKGINFO() *raiou.PKGINFO {
	return p.info
}

// Path はパッケージファイルのパスを返します。
func (p *BinaryPackage) Path() string {
	return p.path
}
