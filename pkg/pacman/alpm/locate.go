package alpm

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Hayao0819/Kamisato/pkg/pacman/makepkgconf"
)

// MakepkgPkgDest returns PKGDEST from makepkg.conf, where a `-U`-installed foreign
// package lands (the pacman cache does not). Empty when PKGDEST is unset.
func MakepkgPkgDest() []string {
	cfg, err := makepkgconf.Read()
	if err != nil || cfg.PKGDEST == "" {
		return nil
	}
	return []string{cfg.PKGDEST}
}

// FilterForeign keeps only the names present in the foreign set.
func FilterForeign(names []string, foreign map[string]bool) []string {
	var out []string
	for _, n := range names {
		if foreign[n] {
			out = append(out, n)
		}
	}
	return out
}

// pkgFileTail matches the arch/.pkg.tar/suffix tail of a package file name;
// the dash-free arch and end anchor exclude look-alikes, .sig, and .part files.
var pkgFileTail = regexp.MustCompile(`^[^-]+\.pkg\.tar(\.[A-Za-z0-9]+)?$`)

// FindCachedPackage finds the built file for name-version; the prefix and strict
// tail exclude signatures, partial downloads, and look-alikes.
func FindCachedPackage(dirs []string, name, version string) (string, bool) {
	prefix := name + "-" + version + "-"
	for _, d := range dirs {
		matches, _ := filepath.Glob(filepath.Join(d, prefix+"*"))
		for _, m := range matches {
			if pkgFileTail.MatchString(strings.TrimPrefix(filepath.Base(m), prefix)) {
				return m, true
			}
		}
	}
	return "", false
}
