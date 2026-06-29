package pkg

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

// BinaryPackageMeta is everything the repository database needs from a built
// package file: the values repo-add records in a desc entry but that are not in
// the .PKGINFO (the stored filename, the file's own size and sha256), the parsed
// .PKGINFO, and the member list for the files database.
type BinaryPackageMeta struct {
	Filename string
	CSize    int64
	SHA256   string
	Info     *raiou.PKGINFO
	Files    []string
}

// ReadBinaryPackageMeta reads a package file in a single pass: the raw bytes are
// teed into a hasher (the desc %SHA256SUM%) while the decompressed tar is walked
// for .PKGINFO and the %FILES% member list. %CSIZE% is the file size from stat.
func ReadBinaryPackageMeta(pkgPath string) (*BinaryPackageMeta, error) {
	f, err := os.Open(pkgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open package: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat package: %w", err)
	}

	h := sha256.New()
	tee := io.TeeReader(f, h)

	var info *raiou.PKGINFO
	var files []string
	err = walkPackageTar(tee, func(hdr *tar.Header, content io.Reader) (bool, error) {
		if hdr.Name == ".PKGINFO" {
			parsed, perr := raiou.ParsePkginfo(content)
			if perr != nil {
				return false, fmt.Errorf("failed to parse .PKGINFO: %w", perr)
			}
			info = parsed
			return false, nil
		}
		// The files database lists every member except top-level dotfiles
		// (.PKGINFO/.MTREE/.BUILDINFO/.INSTALL/.CHANGELOG), matching repo-add's
		// `bsdtar --exclude='^.*'` (a leading-dot anchor, so nested dotfiles stay).
		if !strings.HasPrefix(hdr.Name, ".") {
			files = append(files, hdr.Name)
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, fmt.Errorf(".PKGINFO not found in archive")
	}

	// The tar walk stops at the tar EOF marker, before the compression footer;
	// drain the rest so the hash covers the entire file (matching sha256sum).
	if _, err := io.Copy(io.Discard, tee); err != nil {
		return nil, fmt.Errorf("failed to read package: %w", err)
	}

	slices.Sort(files)
	files = slices.Compact(files)
	return &BinaryPackageMeta{
		Filename: filepath.Base(pkgPath),
		CSize:    fi.Size(),
		SHA256:   hex.EncodeToString(h.Sum(nil)),
		Info:     info,
		Files:    files,
	}, nil
}
