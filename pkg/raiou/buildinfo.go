package raiou

import (
	"io"
	"strings"
)

// BUILDINFO models the pacman .BUILDINFO metadata embedded in a built package.
// Only the provenance fields ayato gates on are decoded; the on-disk format is
// the same key = value grammar as .PKGINFO.
type BUILDINFO struct {
	Format   string `json:"format" yml:"format" toml:"format"`
	BuildDir string `json:"builddir" yml:"builddir" toml:"builddir"`
}

func ParseBuildinfoString(data string) (*BUILDINFO, error) {
	return ParseBuildinfo(strings.NewReader(data))
}

// ParseBuildinfo parses a .BUILDINFO. Unknown keys are ignored on purpose: the
// format grows fields across releases and ayato only reads the provenance ones.
func ParseBuildinfo(r io.Reader) (*BUILDINFO, error) {
	lines, err := readLines(r)
	if err != nil {
		return nil, err
	}
	kvs, err := parseKeyValues(lines)
	if err != nil {
		return nil, err
	}
	b := &BUILDINFO{}
	for _, kv := range kvs {
		switch kv.Key() {
		case "format":
			b.Format = kv.Value()
		case "builddir":
			b.BuildDir = kv.Value()
		}
	}
	return b, nil
}
