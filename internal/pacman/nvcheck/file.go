package nvcheck

import (
	"fmt"
	"os"

	toml "github.com/pelletier/go-toml/v2"
)

// File is one parsed .nvchecker.toml: a single [pkgbase] table describing the
// package's upstream source (the subset of nvchecker's schema this package
// supports).
type File struct {
	Pkgbase string
	Spec    Spec
}

type rawEntry struct {
	Source       string `toml:"source"`
	Github       string `toml:"github"`
	UseMaxTag    bool   `toml:"use_max_tag"`
	Git          string `toml:"git"`
	Pypi         string `toml:"pypi"`
	Archpkg      string `toml:"archpkg"`
	Aur          string `toml:"aur"`
	StripRelease bool   `toml:"strip_release"`
	URL          string `toml:"url"`
	Regex        string `toml:"regex"`
	Prefix       string `toml:"prefix"`
	FromPattern  string `toml:"from_pattern"`
	ToPattern    string `toml:"to_pattern"`
}

// LoadFile reads and parses the .nvchecker.toml at path.
func LoadFile(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	return ParseFile(path, data)
}

// ParseFile parses .nvchecker.toml content; path is used in error messages
// only. An unsupported source kind is not rejected here — NewSource reports it
// so it surfaces as a per-package check error instead of a silent skip.
func ParseFile(path string, data []byte) (File, error) {
	var tables map[string]rawEntry
	if err := toml.Unmarshal(data, &tables); err != nil {
		return File{}, fmt.Errorf("nvcheck: parse %s: %w", path, err)
	}
	delete(tables, "__config__")
	if len(tables) != 1 {
		return File{}, fmt.Errorf("nvcheck: %s must have exactly one [pkgbase] table, got %d", path, len(tables))
	}
	var pkgbase string
	var e rawEntry
	for k, v := range tables {
		pkgbase, e = k, v
	}
	spec := Spec{
		Kind:         e.Source,
		Repo:         e.Github,
		URL:          e.URL,
		Regex:        e.Regex,
		Prefix:       e.Prefix,
		UseMaxTag:    e.UseMaxTag,
		StripRelease: e.StripRelease,
		FromPattern:  e.FromPattern,
		ToPattern:    e.ToPattern,
	}
	switch e.Source {
	case "git":
		spec.URL = e.Git
	case "pypi":
		spec.Package = orDefault(e.Pypi, pkgbase)
	case "archpkg":
		spec.Package = orDefault(e.Archpkg, pkgbase)
	case "aur":
		spec.Package = orDefault(e.Aur, pkgbase)
	}
	return File{Pkgbase: pkgbase, Spec: spec}, nil
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
