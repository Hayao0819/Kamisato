package hookcmd

import (
	"errors"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// makepkgPkgDestScript sources makepkg's config files in makepkg's own order
// (load_makepkg_config) and prints the resolved PKGDEST, so variable expansion
// and includes are interpreted by bash exactly as makepkg would, not guessed.
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

// makepkgPkgDest returns the PKGDEST makepkg would write a built package to, by
// actually running bash to evaluate makepkg.conf (a pacman-hook system always has
// makepkg and bash). That is where a `-U`-installed foreign package can be found;
// the pacman cache cannot. Empty when PKGDEST is unset (built packages stay in
// the build dir, unknowable to a hook that only gets package names).
func makepkgPkgDest() []string {
	out, err := exec.Command("bash", "-c", makepkgPkgDestScript).Output()
	if err != nil {
		return nil
	}
	if dest := strings.TrimSpace(string(out)); dest != "" {
		return []string{dest}
	}
	return nil
}

// foreignPackages returns the set of installed packages no sync repo provides
// (AUR or locally built). pacman -Qmq exits 1 with empty stdout AND empty stderr
// when none are installed (a normal state); a genuine failure writes to stderr,
// so only the no-match signature is treated as an empty set.
func foreignPackages() (map[string]bool, error) {
	out, err := exec.Command("pacman", "-Qmq").Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 1 && len(out) == 0 && len(ee.Stderr) == 0 {
			return map[string]bool{}, nil
		}
		return nil, utils.WrapErr(err, "pacman -Qmq")
	}
	set := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			set[line] = true
		}
	}
	return set, nil
}

func filterForeign(names []string, foreign map[string]bool) []string {
	var out []string
	for _, n := range names {
		if foreign[n] {
			out = append(out, n)
		}
	}
	return out
}

// installedVersion returns the version pacman records for an installed package,
// which (with the name) pins the exact built file in the cache.
func installedVersion(name string) (string, error) {
	out, err := exec.Command("pacman", "-Q", name).Output()
	if err != nil {
		return "", utils.WrapErr(err, "pacman -Q "+name)
	}
	fields := strings.Fields(string(out))
	if len(fields) < 2 {
		return "", utils.NewErrf("unexpected 'pacman -Q' output for %s", name)
	}
	return fields[1], nil
}

// pkgFileTail matches what must follow the name-version- prefix of a built
// package file: a single arch field (no dash), .pkg.tar, and at most one
// compression suffix. The arch field having no dash stops a different package
// whose name+version concatenation aligns (e.g. foo-1.0-1- matching a foo-1.0
// build) from matching, and the end anchor rejects .sig and .part sidecars.
var pkgFileTail = regexp.MustCompile(`^[^-]+\.pkg\.tar(\.[A-Za-z0-9]+)?$`)

// findCachedPackage finds the built package file for name-version in the cache
// dirs. The name-version- prefix plus the strict tail pins exactly the file for
// this package, excluding signatures, partial downloads, and look-alikes.
func findCachedPackage(dirs []string, name, version string) (string, bool) {
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
