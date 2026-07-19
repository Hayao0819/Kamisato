package pkg

import (
	"fmt"
	"strings"
)

// ArtifactCoordinates are the unambiguous right-split fields in a conventional
// package filename. Package names may contain dashes; version and release may
// not.
type ArtifactCoordinates struct {
	Name    string
	Version string
	Release string
	Arch    string
}

func (c ArtifactCoordinates) FullVersion() string {
	return c.Version + "-" + c.Release
}

func (c ArtifactCoordinates) IsAny() bool {
	return c.Arch == "any"
}

// MatchesMetadata reports whether the coordinates agree with .PKGINFO fields.
func (c ArtifactCoordinates) MatchesMetadata(name, version, arch string) bool {
	if c.Name != name || c.Arch != arch {
		return false
	}
	fileVersion := c.FullVersion()
	// Older Kamisato uploads normalized the canonical epoch ':' to '_'. Retain
	// compatibility with those already-published filenames.
	return fileVersion == version ||
		fileVersion == strings.ReplaceAll(version, ":", "_")
}

// Coordinates extracts pkgname, pkgver, pkgrel, and arch by splitting the stem
// from the right.
func (a Artifact) Coordinates() (ArtifactCoordinates, error) {
	parts := strings.Split(a.stem, "-")
	if len(parts) < 4 {
		return ArtifactCoordinates{}, fmt.Errorf(
			"%w: %q has no pkgname-pkgver-pkgrel-arch fields",
			ErrInvalidArtifact,
			a.filename,
		)
	}
	last := len(parts) - 1
	coordinates := ArtifactCoordinates{
		Name:    strings.Join(parts[:last-2], "-"),
		Version: parts[last-2],
		Release: parts[last-1],
		Arch:    parts[last],
	}
	if !validPackageName(coordinates.Name) ||
		!validVersionField(coordinates.Version) ||
		!validVersionField(coordinates.Release) ||
		!validArch(coordinates.Arch) {
		return ArtifactCoordinates{}, fmt.Errorf(
			"%w: %q has invalid package coordinates",
			ErrInvalidArtifact,
			a.filename,
		)
	}
	return coordinates, nil
}

func validArch(arch string) bool {
	if arch == "" {
		return false
	}
	for _, character := range arch {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '_' {
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
	for _, character := range name {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			strings.ContainsRune("@._+-", character) {
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
	for _, character := range field {
		if character <= ' ' || character == '\u007f' {
			return false
		}
	}
	return true
}
