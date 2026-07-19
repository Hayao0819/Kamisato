// Package pkgfile defines the filename grammar for binary pacman packages.
//
// A package artifact has two useful levels of validation:
//   - Parse identifies a supported package archive (or its detached signature).
//   - File.Coordinates validates and extracts the conventional
//     pkgname-pkgver-pkgrel-arch fields.
//
// Keeping those operations separate is intentional. Build backends may collect a
// minimally named archive such as "result.pkg.tar.zst", while publication and
// repository routing require a fully structured filename.
package pkgfile

import (
	"errors"
	"fmt"
	"path"
	"strings"
)

// ErrInvalid is returned when a name is not a supported package artifact.
var ErrInvalid = errors.New("invalid pacman package filename")

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

// SupportedArchiveSuffixes returns a copy of the package suffix allowlist. It is
// suitable for advertising server capabilities to clients.
func SupportedArchiveSuffixes() []string {
	suffixes := make([]string, 0, len(archiveFormats))
	for _, format := range archiveFormats {
		suffixes = append(suffixes, format.suffix)
	}
	return suffixes
}

// File is a supported package archive or its detached signature.
type File struct {
	filename        string
	archiveFilename string
	stem            string
	suffix          string
	compression     Compression
	signature       bool
}

// Parse validates a base filename as a supported package archive or detached
// signature. It deliberately does not require conventional package coordinates;
// call Coordinates when routing or publication needs them.
func Parse(name string) (File, error) {
	if name == "" || path.Base(name) != name || strings.ContainsRune(name, '\\') {
		return File{}, fmt.Errorf("%w: %q must be a base filename", ErrInvalid, name)
	}

	archiveName := name
	signature := strings.HasSuffix(archiveName, ".sig")
	if signature {
		archiveName = strings.TrimSuffix(archiveName, ".sig")
	}
	for _, format := range archiveFormats {
		if len(archiveName) <= len(format.suffix) || !strings.HasSuffix(archiveName, format.suffix) {
			continue
		}
		return File{
			filename:        name,
			archiveFilename: archiveName,
			stem:            strings.TrimSuffix(archiveName, format.suffix),
			suffix:          format.suffix,
			compression:     format.compression,
			signature:       signature,
		}, nil
	}
	return File{}, fmt.Errorf("%w: %q has an unsupported archive suffix", ErrInvalid, name)
}

// IsArchive reports whether name is a supported package archive (not a
// signature).
func IsArchive(name string) bool {
	file, err := Parse(name)
	return err == nil && !file.IsSignature()
}

// IsArtifact reports whether name is a supported package archive or its detached
// signature.
func IsArtifact(name string) bool {
	_, err := Parse(name)
	return err == nil
}

// IsAny reports whether name is a structured architecture-independent package
// archive or detached signature.
func IsAny(name string) bool {
	file, err := Parse(name)
	if err != nil {
		return false
	}
	coords, err := file.Coordinates()
	return err == nil && coords.IsAny()
}

// Filename returns the filename exactly as parsed.
func (f File) Filename() string { return f.filename }

// ArchiveFilename returns the unsigned package archive filename.
func (f File) ArchiveFilename() string { return f.archiveFilename }

// SignatureFilename returns the conventional detached-signature filename.
func (f File) SignatureFilename() string { return f.archiveFilename + ".sig" }

// ArchiveSuffix returns the complete package suffix, such as ".pkg.tar.zst".
func (f File) ArchiveSuffix() string { return f.suffix }

// Compression returns the package archive's compression format.
func (f File) Compression() Compression { return f.compression }

// IsSignature reports whether the parsed artifact is a detached signature.
func (f File) IsSignature() bool { return f.signature }

// Coordinates are the unambiguous right-split fields in a conventional package
// filename. Package names may contain dashes; version and release may not.
type Coordinates struct {
	Name    string
	Version string
	Release string
	Arch    string
}

// FullVersion returns the PKGINFO-style "pkgver-pkgrel" value represented by the
// filename.
func (c Coordinates) FullVersion() string { return c.Version + "-" + c.Release }

// IsAny reports whether these coordinates describe an architecture-independent
// package.
func (c Coordinates) IsAny() bool { return c.Arch == "any" }

// MatchesMetadata reports whether the coordinates agree with .PKGINFO fields.
func (c Coordinates) MatchesMetadata(name, version, arch string) bool {
	if c.Name != name || c.Arch != arch {
		return false
	}
	fileVersion := c.FullVersion()
	// makepkg's canonical epoch separator is ':'. Older Kamisato uploads
	// normalized it to '_', so retain read/publication compatibility with those
	// already-stored filenames while accepting makepkg output verbatim.
	return fileVersion == version || fileVersion == strings.ReplaceAll(version, ":", "_")
}

// Coordinates extracts pkgname, pkgver, pkgrel, and arch by splitting the stem
// from the right. Pacman package names may contain dashes, so a left split is not
// correct.
func (f File) Coordinates() (Coordinates, error) {
	parts := strings.Split(f.stem, "-")
	if len(parts) < 4 {
		return Coordinates{}, fmt.Errorf("%w: %q has no pkgname-pkgver-pkgrel-arch fields", ErrInvalid, f.filename)
	}
	last := len(parts) - 1
	coords := Coordinates{
		Name:    strings.Join(parts[:last-2], "-"),
		Version: parts[last-2],
		Release: parts[last-1],
		Arch:    parts[last],
	}
	if !validPackageName(coords.Name) || !validVersionField(coords.Version) ||
		!validVersionField(coords.Release) || !validArch(coords.Arch) {
		return Coordinates{}, fmt.Errorf("%w: %q has invalid package coordinates", ErrInvalid, f.filename)
	}
	return coords, nil
}

func validArch(arch string) bool {
	if arch == "" {
		return false
	}
	for _, r := range arch {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

func validPackageName(name string) bool {
	if name == "" || name[0] == '.' || name[0] == '-' {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || strings.ContainsRune("@._+-", r) {
			continue
		}
		return false
	}
	return true
}

func validVersionField(field string) bool {
	if field == "" {
		return false
	}
	for _, r := range field {
		if r <= ' ' || r == '\u007f' {
			return false
		}
	}
	return true
}
