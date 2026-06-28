package hook

import (
	"bufio"
	"os"
	"strings"
)

// StdinTargets reads the package targets pacman feeds a NeedsTargets hook, one
// per line, on stdin. It returns nil when stdin is a terminal (a manual run with
// no piped targets), so callers can fall back to positional arguments.
func StdinTargets() []string {
	if info, err := os.Stdin.Stat(); err != nil || info.Mode()&os.ModeCharDevice != 0 {
		return nil
	}
	var names []string
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		if n := strings.TrimSpace(sc.Text()); n != "" {
			names = append(names, n)
		}
	}
	return names
}
