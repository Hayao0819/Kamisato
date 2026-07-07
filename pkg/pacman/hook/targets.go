package hook

import (
	"bufio"
	"os"
	"strings"
)

// StdinTargets reads newline-separated pacman hook targets from stdin;
// returns nil when stdin is a terminal so callers can fall back to positional args.
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
