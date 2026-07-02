package alpm

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// makepkgPkgDestScript sources makepkg's config files in makepkg's own order so
// bash interprets expansion and includes exactly as makepkg would, not guessed.
const makepkgPkgDestScript = `confdir=/etc
[[ -r $confdir/makepkg.conf ]] && source "$confdir/makepkg.conf"
if [[ -d $confdir/makepkg.conf.d ]]; then
  for f in "$confdir/makepkg.conf.d"/*.conf; do
    [[ -r $f ]] && source "$f"
  done
fi
if [[ -r ${XDG_CONFIG_HOME:-$HOME/.config}/pacman/makepkg.conf ]]; then
  source "${XDG_CONFIG_HOME:-$HOME/.config}/pacman/makepkg.conf"
elif [[ -r $HOME/.makepkg.conf ]]; then
  source "$HOME/.makepkg.conf"
fi
printf '%s' "${PKGDEST:-}"`

// MakepkgPkgDest returns PKGDEST by running bash on makepkg.conf, since that is
// where a `-U`-installed foreign package lands and the pacman cache is not. Empty
// when PKGDEST is unset.
func MakepkgPkgDest() []string {
	out, err := exec.Command("bash", "-c", makepkgPkgDestScript).Output()
	if err != nil {
		return nil
	}
	if dest := strings.TrimSpace(string(out)); dest != "" {
		return []string{dest}
	}
	return nil
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
