package alpm

import (
	"errors"
	"os/exec"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// Deptest returns the subset of deps not satisfied by the configured sync repos,
// via `pacman -T`. Unlike a name-set lookup, pacman -T resolves provides and
// version constraints (e.g. "glibc>=2.38"). pacman -T exits 127 when some deps
// are unsatisfied — printing them on stdout — which is the normal "missing" case,
// not an error; exit 0 means all are satisfied.
func Deptest(deps []string) ([]string, error) {
	if len(deps) == 0 {
		return nil, nil
	}
	out, err := exec.Command("pacman", append([]string{"-T"}, deps...)...).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 127 {
			return strings.Fields(string(out)), nil
		}
		return nil, utils.WrapErr(err, "pacman -T")
	}
	return nil, nil
}
