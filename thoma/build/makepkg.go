package build

import (
	"os"
	"os/exec"
)

func RealMakepkg(configured string) string {
	self, _ := os.Executable()
	var candidates []string
	if path := os.Getenv("THOMA_MAKEPKG"); path != "" {
		candidates = append(candidates, path)
	}
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
