package pkg

import (
	"errors"
	"fmt"
	"path"
	"strings"
)

// ErrInvalidArtifact is returned when a name is not a supported pacman package
// archive or detached signature.
var ErrInvalidArtifact = errors.New("invalid pacman package artifact")

// Compression is the compression suffix of a pacman package archive.
type Compression string

const (
	CompressionNone  Compression = ""
	CompressionZstd  Compression = "zst"
	CompressionXZ    Compression = "xz"
	CompressionGzip  Compression = "gz"
	CompressionBzip2 Compression = "bz2"
	CompressionLRZip Compression = "lrz"
	CompressionLZO   Compression = "lzo"
	CompressionLZ4   Compression = "lz4"
	CompressionLzip  Compression = "lz"
	CompressionZ     Compression = "Z"
)

type archiveFormat struct {
	suffix      string
	compression Compression
}

// Longest suffixes come first so an uncompressed ".pkg.tar" never consumes a
// compressed package prefix.
var archiveFormats = [...]archiveFormat{
	{suffix: ".pkg.tar.zst", compression: CompressionZstd},
	{suffix: ".pkg.tar.bz2", compression: CompressionBzip2},
	{suffix: ".pkg.tar.lrz", compression: CompressionLRZip},
	{suffix: ".pkg.tar.lzo", compression: CompressionLZO},
	{suffix: ".pkg.tar.lz4", compression: CompressionLZ4},
	{suffix: ".pkg.tar.xz", compression: CompressionXZ},
	{suffix: ".pkg.tar.gz", compression: CompressionGzip},
	{suffix: ".pkg.tar.lz", compression: CompressionLzip},
	{suffix: ".pkg.tar.Z", compression: CompressionZ},
	{suffix: ".pkg.tar", compression: CompressionNone},
}

// Artifact is a supported package archive or its detached signature. Parsing
// recognizes minimally named build outputs; Coordinates applies the stricter
// publication filename grammar.
type Artifact struct {
	filename        string
	archiveFilename string
	stem            string
	suffix          string
	compression     Compression
	signature       bool
}

// ParseArtifact validates a base filename as a supported package archive or
// detached signature.
func ParseArtifact(name string) (Artifact, error) {
	if name == "" || path.Base(name) != name || strings.ContainsRune(name, '\\') {
		return Artifact{}, fmt.Errorf("%w: %q must be a base filename", ErrInvalidArtifact, name)
	}

	archiveName := name
	signature := strings.HasSuffix(archiveName, ".sig")
	if signature {
		archiveName = strings.TrimSuffix(archiveName, ".sig")
	}
	for _, format := range archiveFormats {
		if len(archiveName) <= len(format.suffix) ||
			!strings.HasSuffix(archiveName, format.suffix) {
			continue
		}
		return Artifact{
			filename:        name,
			archiveFilename: archiveName,
			stem:            strings.TrimSuffix(archiveName, format.suffix),
			suffix:          format.suffix,
			compression:     format.compression,
			signature:       signature,
		}, nil
	}
	return Artifact{}, fmt.Errorf(
		"%w: %q has an unsupported archive suffix",
		ErrInvalidArtifact,
		name,
	)
}

// SupportedArchiveSuffixes returns a caller-owned copy of the package suffix
// allowlist.
func SupportedArchiveSuffixes() []string {
	suffixes := make([]string, 0, len(archiveFormats))
	for _, format := range archiveFormats {
		suffixes = append(suffixes, format.suffix)
	}
	return suffixes
}

func IsArchive(name string) bool {
	artifact, err := ParseArtifact(name)
	return err == nil && !artifact.IsSignature()
}

func IsArtifact(name string) bool {
	_, err := ParseArtifact(name)
	return err == nil
}

func IsAny(name string) bool {
	artifact, err := ParseArtifact(name)
	if err != nil {
		return false
	}
	coordinates, err := artifact.Coordinates()
	return err == nil && coordinates.IsAny()
}

func (a Artifact) Filename() string { return a.filename }

func (a Artifact) ArchiveFilename() string { return a.archiveFilename }

func (a Artifact) SignatureFilename() string { return a.archiveFilename + ".sig" }

func (a Artifact) ArchiveSuffix() string { return a.suffix }

func (a Artifact) Compression() Compression { return a.compression }

func (a Artifact) IsSignature() bool { return a.signature }
