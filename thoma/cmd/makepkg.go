package cmd

import (
	"os"
	"os/exec"
)

// realMakepkg resolves the local executable in the command adapter, before the
// workflow runs. It must never resolve back to the shim itself.
func realMakepkg(configured string) string {
	self, _ := os.Executable()
	var candidates []string
	if configured != "" {
		candidates = append(candidates, configured)
	}
	if path, err := exec.LookPath("makepkg"); err == nil {
		candidates = append(candidates, path)
	}
	candidates = append(candidates, "/usr/bin/makepkg")
	for _, candidate := range candidates {
		if self != "" && sameFile(candidate, self) {
			continue
		}
		return candidate
	}
	return "/usr/bin/makepkg"
}

func sameFile(a, b string) bool {
	first, err := os.Stat(a) //nolint:gosec // Candidates are resolved executable paths.
	if err != nil {
		return false
	}
	second, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(first, second)
}
