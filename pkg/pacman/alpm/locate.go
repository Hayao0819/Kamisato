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

// pkgFileTail matches the part after the name-version- prefix of a built package
// file: a dash-free arch field, .pkg.tar, one optional compression suffix. The
// dash-free arch rejects look-alike names; the end anchor rejects .sig/.part.
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
