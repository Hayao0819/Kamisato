package alpm

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Deptest returns the subset of deps unsatisfied by locally installed packages via `pacman -T`.
// pacman -T resolves provides/version constraints but consults only the local db;
// exit 127 (some missing, printed on stdout) is the normal case, not an error.
func Deptest(deps []string) ([]string, error) {
	if len(deps) == 0 {
		return nil, nil
	}
	out, err := exec.Command("pacman", append([]string{"-T"}, deps...)...).Output() //nolint:gosec // fixed program, argv passed as separate args (no shell)
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 127 {
			return strings.Fields(string(out)), nil
		}
		return nil, fmt.Errorf("pacman -T: %w", err)
	}
	return nil, nil
}
