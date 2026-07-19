package alpm

import (
	"path/filepath"

	"github.com/Hayao0819/Kamisato/pkg/pacman/makepkgconf"
	"github.com/Hayao0819/Kamisato/pkg/pacman/pkgfile"
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

// FindCachedPackage finds the built file for name-version; the prefix and strict
// tail exclude signatures, partial downloads, and look-alikes.
func FindCachedPackage(dirs []string, name, version string) (string, bool) {
	for _, d := range dirs {
		matches, _ := filepath.Glob(filepath.Join(d, name+"-*"))
		for _, m := range matches {
			file, err := pkgfile.Parse(filepath.Base(m))
			if err != nil || file.IsSignature() {
				continue
			}
			coords, err := file.Coordinates()
			if err == nil && coords.MatchesMetadata(name, version, coords.Arch) {
				return m, true
			}
		}
	}
	return "", false
}
