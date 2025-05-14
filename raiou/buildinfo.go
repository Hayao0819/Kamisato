package raiou

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// BUILDINFO represents the parsed .BUILDINFO file (version 2).
type BUILDINFO struct {
	Format            int
	PkgName           string
	PkgBase           string
	PkgVer            string
	PkgArch           string
	PkgbuildSHA256Sum string
	Packager          string
	BuildDate         int64
	BuildDir          string
	StartDir          string
	BuildTool         string
	BuildToolVer      string
	BuildEnv          []string
	Options           []string
	Installed         []string
	XData             map[string]string // For any unrecognized keywords
}

// NewBUILDINFO creates a new BUILDINFO struct.
func NewBUILDINFO() *BUILDINFO {
	return &BUILDINFO{
		BuildEnv:  make([]string, 0),
		Options:   make([]string, 0),
		Installed: make([]string, 0),
		XData:     make(map[string]string),
	}
}

// ParseBuildinfoFile reads a .BUILDINFO file from the given path and returns a BUILDINFO struct.
func ParseBuildinfoFile(path string) (*BUILDINFO, error) {
	r, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer r.Close()
	return ParseBuildinfo(r)
}

// ParseBuildinfoString reads a .BUILDINFO content from the given string and returns a BUILDINFO struct.
func ParseBuildinfoString(data string) (*BUILDINFO, error) {
	r := strings.NewReader(data)
	return ParseBuildinfo(r)
}

// ParseBuildinfo reads a .BUILDINFO file from the given io.Reader and returns a BUILDINFO struct.
func ParseBuildinfo(r io.Reader) (*BUILDINFO, error) {
	b := NewBUILDINFO()
	lines, err := readLines(r)
	if err != nil {
		return nil, err
	}

	keyValues, err := parseKeyValues(lines)
	if err != nil {
		return nil, fmt.Errorf("error parsing key-value pairs: %w", err)
	}

	if err := b.parseKeyValues(keyValues); err != nil {
		return nil, err
	}

	if b.Format != 2 {
		return nil, fmt.Errorf("unsupported BUILDINFO format version: %d", b.Format)
	}

	return b, nil
}

func (b *BUILDINFO) parseKeyValues(kvs []keyValue) error {
	for _, kv := range kvs {
		key := kv.Key()
		value := kv.Value()

		switch key {
		case "format":
			format, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid format: %s", value)
			}
			b.Format = format
		case "pkgname":
			b.PkgName = value
		case "pkgbase":
			b.PkgBase = value
		case "pkgver":
			b.PkgVer = value
		case "pkgarch":
			b.PkgArch = value
		case "pkgbuild_sha256sum":
			if len(value) != 64 || !isHex(value) {
				return fmt.Errorf("invalid pkgbuild_sha256sum: %s", value)
			}
			b.PkgbuildSHA256Sum = value
		case "packager":
			b.Packager = value
		case "builddate":
			buildDate, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid builddate: %s", value)
			}
			b.BuildDate = buildDate
		case "builddir":
			b.BuildDir = value
		case "startdir":
			b.StartDir = value
		case "buildtool":
			b.BuildTool = value
		case "buildtoolver":
			b.BuildToolVer = value
		case "buildenv":
			b.BuildEnv = append(b.BuildEnv, value)
		case "options":
			b.Options = append(b.Options, value)
		case "installed":
			b.Installed = append(b.Installed, value)
		default:
			b.XData[key] = value
		}
	}
	return nil
}

func isHex(s string) bool {
	for _, r := range s {
		if !('0' <= r && r <= '9') && !('a' <= r && r <= 'f') {
			return false
		}
	}
	return true
}
