// Package makepkgconf reads makepkg's configuration by sourcing it with bash.
// That is the only faithful way: makepkg.conf is a bash script with includes and
// variable expansion, not a static key=value file, so a Go parser would guess at
// what bash resolves exactly.
package makepkgconf

import (
	"os/exec"
	"strings"
)

// Conf holds the makepkg.conf fields Kamisato consumes. An unset field is "".
type Conf struct {
	CARCH   string
	CHOST   string
	PKGDEST string
	PKGEXT  string
}

// sourceChain sources makepkg's config in makepkg's own order so bash applies includes and expansion exactly as makepkg would.
const sourceChain = `confdir=/etc
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
`

// printVars emits the consumed fields one per line, in Conf field order.
const printVars = `printf '%s\n' "${CARCH:-}" "${CHOST:-}" "${PKGDEST:-}" "${PKGEXT:-}"`

// Read resolves makepkg's config from the system default file chain (the same
// files makepkg itself reads).
func Read() (*Conf, error) {
	out, err := exec.Command("bash", "-c", sourceChain+printVars).Output()
	if err != nil {
		return nil, err
	}
	return parse(out), nil
}

// ReadFile resolves makepkg's config from a single file; a missing file yields an all-empty Conf
// rather than an error, mirroring makepkg's tolerance of an absent override.
func ReadFile(path string) (*Conf, error) {
	// path rides in as a bash positional ($1), never interpolated into the script,
	// so it cannot inject shell; sourcing the referenced makepkg.conf is intended.
	out, err := exec.Command("bash", "-c", `[[ -r "$1" ]] && source "$1"; `+printVars, "makepkgconf", path).Output() //nolint:gosec // path is a positional arg, not interpolated
	if err != nil {
		return nil, err
	}
	return parse(out), nil
}

func parse(out []byte) *Conf {
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	at := func(i int) string {
		if i < len(lines) {
			return strings.TrimSpace(lines[i])
		}
		return ""
	}
	return &Conf{
		CARCH:   at(0),
		CHOST:   at(1),
		PKGDEST: at(2),
		PKGEXT:  at(3),
	}
}
