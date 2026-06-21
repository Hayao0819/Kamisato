// Package pkg models two kinds of packages as domain types: the build target
// (SourcePackage) and the build output (BinaryPackage). Metadata parsing is
// delegated to pkg/raiou.
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

// SourcePackage is a build-target package from a .SRCINFO / PKGBUILD directory.
type SourcePackage struct {
	dir  string
	info *raiou.SRCINFO
}

// OpenSourcePackage reads the directory's .SRCINFO and returns a SourcePackage.
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

// SRCINFO returns the parsed .SRCINFO.
func (p *SourcePackage) SRCINFO() *raiou.SRCINFO {
	return p.info
}

// Dir returns the source directory path.
func (p *SourcePackage) Dir() string {
	return p.dir
}

// BinaryPackage is a build-output package from a .pkg.tar or a repository DB desc.
type BinaryPackage struct {
	path string
	info *raiou.PKGINFO
}

// OpenBinaryPackage opens a package file and returns a BinaryPackage.
func OpenBinaryPackage(binPath string) (*BinaryPackage, error) {
	file, err := os.Open(binPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	return ReadBinaryPackage(binPath, file)
}

// ReadBinaryPackage reads .PKGINFO from r and returns a BinaryPackage.
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

// PKGINFO returns the parsed .PKGINFO.
func (p *BinaryPackage) PKGINFO() *raiou.PKGINFO {
	return p.info
}

// Path returns the package file path.
func (p *BinaryPackage) Path() string {
	return p.path
}
