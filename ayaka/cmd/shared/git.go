package shared

import (
	"os/exec"
	"strings"
)

// GitRootDir returns the root directory of the git repository that contains dir.
// If dir is not inside a git repository, an error is returned.
func GitRootDir(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
