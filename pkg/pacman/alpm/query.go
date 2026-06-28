package alpm

import (
	"errors"
	"os/exec"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// InstalledVersion returns the version pacman records for an installed package
// (pacman -Q <name>).
func InstalledVersion(name string) (string, error) {
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

// InstalledVersions maps every installed package name to its version (pacman -Q).
func InstalledVersions() (map[string]string, error) {
	out, err := exec.Command("pacman", "-Q").Output()
	if err != nil {
		return nil, utils.WrapErr(err, "pacman -Q")
	}
	m := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if f := strings.Fields(line); len(f) >= 2 {
			m[f[0]] = f[1]
		}
	}
	return m, nil
}

// ForeignPackages returns the set of installed packages no sync repo provides
// (AUR or locally built), via pacman -Qmq. pacman -Q exits 1 with empty stdout
// AND empty stderr when none are installed — a normal state, not an error.
func ForeignPackages() (map[string]bool, error) {
	out, err := exec.Command("pacman", "-Qmq").Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 1 && len(out) == 0 && len(ee.Stderr) == 0 {
			return map[string]bool{}, nil
		}
		return nil, utils.WrapErr(err, "pacman -Qmq")
	}
	return lineSet(out), nil
}

// SyncPackages returns the set of package names any sync (official) repo
// provides, via pacman -Slq.
func SyncPackages() (map[string]bool, error) {
	out, err := exec.Command("pacman", "-Slq").Output()
	if err != nil {
		return nil, utils.WrapErr(err, "pacman -Slq")
	}
	return lineSet(out), nil
}

func lineSet(out []byte) map[string]bool {
	set := map[string]bool{}
	for _, n := range strings.Fields(string(out)) {
		set[n] = true
	}
	return set
}
