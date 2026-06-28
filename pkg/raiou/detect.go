package raiou

import (
	"bufio"
	"io"
	"strings"
)

type FileType int

const (
	TypeUnknown FileType = iota
	TypePKGINFO
	TypeBUILDINFO
	TypeSRCINFO
)

func (ft FileType) String() string {
	switch ft {
	case TypePKGINFO:
		return "PKGINFO"
	case TypeBUILDINFO:
		return "BUILDINFO"
	case TypeSRCINFO:
		return "SRCINFO"
	default:
		return "Unknown"
	}
}

func DetectType(r io.Reader) (FileType, error) {
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed[0] == '#' {
			continue
		}

		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "format":
			if value == "2" {
				return TypeBUILDINFO, nil
			}
		case "pkgarch", "pkgbuild_sha256sum", "builddir", "startdir", "buildtool", "buildtoolver", "buildenv", "options", "installed":
			return TypeBUILDINFO, nil // keywords specific to BUILDINFO
		case "epoch", "install", "changelog", "md5sums", "sha1sums", "sha224sums", "sha256sums", "sha384sums", "sha512sums", "b2sums", "noextract", "source", "validpgpkeys":
			return TypeSRCINFO, nil // keywords specific to SRCINFO
		default:
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return TypeUnknown, err
	}

	return TypePKGINFO, nil // treat as PKGINFO when none of the above match

}
