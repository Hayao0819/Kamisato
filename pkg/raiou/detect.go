package raiou

import (
	"bufio"
	"io"
	"os"
	"strings"
)

// FileType represents the type of file.
type FileType int

const (
	TypeUnknown FileType = iota
	TypePKGINFO
	TypeBUILDINFO
	TypeSRCINFO
)

// String returns the string representation of the FileType.
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

// DetectType は、io.Readerからファイルのタイプを検出します。
func DetectType(r io.Reader) (FileType, error) {
	scanner := bufio.NewScanner(r)


	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed[0] == '#' {
			continue // 空行とコメント行はスキップ
		}

		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue // 無効な行、スキップ。タイプの判別には役立たない。
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "format":
			if value == "2" {
				return TypeBUILDINFO, nil //  BUILDINFOに固有
			}
		case "pkgarch", "pkgbuild_sha256sum", "builddir", "startdir", "buildtool", "buildtoolver", "buildenv", "options", "installed":
			return TypeBUILDINFO, nil // BUILDINFOに固有のキーワード
		case "epoch", "install", "changelog", "md5sums", "sha1sums", "sha224sums", "sha256sums", "sha384sums", "sha512sums", "b2sums", "noextract", "source", "validpgpkeys":
			return TypeSRCINFO, nil // SRCINFOに固有のキーワード
		default:
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return TypeUnknown, err
	}

	return TypePKGINFO, nil // 上記の条件に合致しない場合はPKGINFOとみなす

}

// DetectTypeFile は、指定されたパスのファイルのタイプを検出します。
// Deprecated: DetectTypeを使用してください。
func DetectTypeFile(path string) (FileType, error) {
	file, err := os.Open(path)
	if err != nil {
		return TypeUnknown, err
	}
	defer file.Close()

	return DetectType(file)
}

// DetectTypeString は、与えられた文字列のタイプを検出します。
// Deprecated: DetectTypeを使用してください。
func DetectTypeString(s string) (FileType, error) {
	return DetectType(strings.NewReader(s))
}
