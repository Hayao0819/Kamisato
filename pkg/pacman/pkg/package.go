// Package pkg owns pacman package artifacts and their metadata: source build
// targets, binary build outputs, package filename grammar, and archive
// inspection. Metadata text parsing is delegated to pkg/raiou.
package pkg

import (
	"archive/tar"
	"fmt"
	"io"
	"path"
	"strings"

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

func (p *SourcePackage) Dir() string {
	return p.dir
}

// BinaryPackage is a build-output package from a .pkg.tar or a repository DB desc.
type BinaryPackage struct {
	path string
	info *raiou.PKGINFO
}

// walkPackageTar decompresses r and calls fn for each tar entry; stop=true ends early.
// Shared by ReadBinaryPackage and ReadBinaryPackageMeta.
func walkPackageTar(r io.Reader, fn func(hdr *tar.Header, content io.Reader) (stop bool, err error)) error {
	decoder, _, err := DetectCompression(r)
	if err != nil {
		return fmt.Errorf("failed to create decoder: %w", err)
	}
	defer decoder.Close()

	tr := tar.NewReader(decoder)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}
		stop, err := fn(hdr, tr)
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
	}
}

// ReadBinaryPackage reads .PKGINFO from r and returns a BinaryPackage.
func ReadBinaryPackage(binPath string, r io.Reader) (*BinaryPackage, error) {
	var pkginfoData string
	err := walkPackageTar(r, func(hdr *tar.Header, content io.Reader) (bool, error) {
		if hdr.Name != ".PKGINFO" {
			return false, nil
		}
		buf := new(strings.Builder)
		if _, err := io.Copy(buf, content); err != nil {
			return false, fmt.Errorf("failed to read .PKGINFO: %w", err)
		}
		pkginfoData = buf.String()
		return true, nil
	})
	if err != nil {
		return nil, err
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

// NewBinaryPackage wraps metadata with its file path; for DB desc packages filePath is the %FILENAME%.
func NewBinaryPackage(filePath string, info *raiou.PKGINFO) *BinaryPackage {
	return &BinaryPackage{path: filePath, info: info}
}

func (p *BinaryPackage) PKGINFO() *raiou.PKGINFO {
	return p.info
}

func (p *BinaryPackage) Path() string {
	return p.path
}
