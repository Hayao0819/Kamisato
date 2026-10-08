package hook

import (
	"bufio"
	"io"
	"os"
	"strings"
)

// ReadTargets reads newline-separated pacman hook targets. Terminal input is
// ignored so an interactive command can fall back to positional arguments.
func ReadTargets(input io.Reader) ([]string, error) {
	if file, ok := input.(*os.File); ok {
		info, err := file.Stat()
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeCharDevice != 0 {
			return nil, nil
		}
	}
	var names []string
	sc := bufio.NewScanner(input)
	for sc.Scan() {
		if n := strings.TrimSpace(sc.Text()); n != "" {
			names = append(names, n)
		}
	}
	return names, sc.Err()
}
